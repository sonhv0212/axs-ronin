// Copyright 2022 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package trie

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie/triedb/hashdb"
	"github.com/ethereum/go-ethereum/trie/triedb/pathdb"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/trie/triestate"
)

// Config defines all necessary options for database.
type Config struct {
	Preimages bool // Flag whether the preimage of trie key is recorded

	HashDB *hashdb.Config // Configs for hash-based scheme
	PathDB *pathdb.Config // Configs for experimental path-based scheme

	// Testing hooks
	OnCommit func(states *triestate.Set) // Hook invoked when commit is performed
}

// backend defines the methods needed to access/update trie nodes in different
// state scheme.
type backend interface {
	// Scheme returns the identifier of used storage scheme.
	Scheme() string

	// Initialized returns an indicator if the state data is already initialized
	// according to the state scheme.
	Initialized(genesisRoot common.Hash) bool

	// Size returns the current storage size of the memory cache in front of the
	// persistent database layer.
	Size() common.StorageSize

	// Update performs a state transition by committing dirty nodes contained
	// in the given set in order to update state from the specified parent to
	// the specified root.
	// The passed in maps(nodes, states) will be retained to avoid copying
	// everything. Therefore, these maps must not be changed afterwards.
	Update(root common.Hash, parent common.Hash, block uint64, nodes *trienode.MergedNodeSet, states *triestate.Set) error

	// DiskDB retrieves the persistent storage backing the trie database.
	DiskDB() ethdb.KeyValueStore

	// Commit writes all relevant trie nodes belonging to the specified state
	// to disk. Report specifies whether logs will be displayed in info level.
	Commit(root common.Hash, report bool) error

	// Close closes the trie database backend and releases all held resources.
	Close() error
}

// Database is the wrapper of the underlying backend which is shared by different
// types of node backend as an entrypoint. It's responsible for all interactions
// relevant with trie nodes and node preimages.
type Database struct {
	config    *Config        // Configuration for trie database
	diskdb    ethdb.Database // Persistent database to store the snapshot
	preimages *preimageStore // The store for caching preimages
	backend   backend        // The backend for managing trie nodes
}

// prepare initializes the database with provided configs, but the
// database backend is still left as nil.
func prepare(diskdb ethdb.Database, config *Config) *Database {
	var preimages *preimageStore
	if config != nil && config.Preimages {
		preimages = newPreimageStore(diskdb)
	}
	return &Database{
		config:    config,
		diskdb:    diskdb,
		preimages: preimages,
	}
}

// HashDefaults represents a config for using hash-based scheme with
// default settings.
var HashDefaults = &Config{
	Preimages: false,
	HashDB:    hashdb.Defaults,
}

// NewDatabase initializes the trie database with default settings, namely
// the legacy hash-based scheme is used by default.
func NewDatabase(diskdb ethdb.Database, config *Config) *Database {
	if config == nil {
		config = HashDefaults
	}
	var preimages *preimageStore
	if config.Preimages {
		preimages = newPreimageStore(diskdb)
	}
	db := &Database{
		config:    config,
		diskdb:    diskdb,
		preimages: preimages,
	}
	if config.HashDB != nil && config.PathDB != nil {
		log.Crit("Both 'hash' and 'path' mode are configured")
	}
	if config.PathDB != nil {
		db.backend = pathdb.New(diskdb, config.PathDB)
	} else {
		// Use hashdb by default
		db.backend = hashdb.New(diskdb, config.HashDB, mptResolver{})
	}
	return db
}

// Reader returns a reader for accessing all trie nodes with provided state root.
// Nil is returned in case the state is not available.
func (db *Database) Reader(blockRoot common.Hash) (Reader, error) {
	switch b := db.backend.(type) {
	case *hashdb.Database:
		return b.Reader(blockRoot)
	case *pathdb.Database:
		return b.Reader(blockRoot)
	}
	return nil, errors.New("unsupported")
}

// Update performs a state transition by committing dirty nodes contained in the
// given set in order to update state from the specified parent to the specified
// root. The held pre-images accumulated up to this point will be flushed in case
// the size exceeds the threshold.
func (db *Database) Update(root common.Hash, parent common.Hash, block uint64, nodes *trienode.MergedNodeSet, states *triestate.Set) error {
	if db.config != nil && db.config.OnCommit != nil {
		db.config.OnCommit(states)
	}
	if db.preimages != nil {
		db.preimages.commit(false)
	}
	return db.backend.Update(root, parent, block, nodes, states)
}

// Commit iterates over all the children of a particular node, writes them out
// to disk. As a side effect, all pre-images accumulated up to this point are
// also written.
func (db *Database) Commit(root common.Hash, report bool) error {
	if db.preimages != nil {
		db.preimages.commit(true)
	}
	return db.backend.Commit(root, report)
}

// Size returns the storage size of dirty trie nodes in front of the persistent
// database and the size of cached preimages.
func (db *Database) Size() (common.StorageSize, common.StorageSize) {
	var (
		storages  common.StorageSize
		preimages common.StorageSize
	)
	storages = db.backend.Size()
	if db.preimages != nil {
		preimages = db.preimages.size()
	}
	return storages, preimages
}

// Initialized returns an indicator if the state data is already initialized
// according to the state scheme.
func (db *Database) Initialized(genesisRoot common.Hash) bool {
	return db.backend.Initialized(genesisRoot)
}

// Scheme returns the node scheme used in the database.
func (db *Database) Scheme() string {
	return db.backend.Scheme()
}

// DiskDB retrieves the persistent storage backing the trie database.
func (db *Database) DiskDB() ethdb.KeyValueStore {
	return db.backend.DiskDB()
}

// Close flushes the dangling preimages to disk and closes the trie database.
// It is meant to be called when closing the blockchain object, so that all
// resources held can be released correctly.
func (db *Database) Close() error {
	if db.preimages != nil {
		db.preimages.commit(true)
	}
	return db.backend.Close()
}

// Cap iteratively flushes old but still referenced trie nodes until the total
// memory usage goes below the given threshold. The held pre-images accumulated
// up to this point will be flushed in case the size exceeds the threshold.
//
// It's only supported by hash-based database and will return an error for others.
func (db *Database) Cap(limit common.StorageSize) error {
	hdb, ok := db.backend.(*hashdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	if db.preimages != nil {
		db.preimages.commit(false)
	}
	return hdb.Cap(limit)
}

// Reference adds a new reference from a parent node to a child node. This function
// is used to add reference between internal trie node and external node(e.g. storage
// trie root), all internal trie nodes are referenced together by database itself.
//
// It's only supported by hash-based database and will return an error for others.
func (db *Database) Reference(root common.Hash, parent common.Hash) error {
	hdb, ok := db.backend.(*hashdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	hdb.Reference(root, parent)
	return nil
}

// Dereference removes an existing reference from a root node. It's only
// supported by hash-based database and will return an error for others.
func (db *Database) Dereference(root common.Hash) error {
	hdb, ok := db.backend.(*hashdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	hdb.Dereference(root)
	return nil
}

// Node retrieves the rlp-encoded node blob with provided node hash. It's
// only supported by hash-based database and will return an error for others.
// Note, this function should be deprecated once ETH66 is deprecated.
func (db *Database) Node(hash common.Hash) ([]byte, error) {
	hdb, ok := db.backend.(*hashdb.Database)
	if !ok {
		return nil, errors.New("not supported")
	}
	return hdb.Node(hash)
}

// Recover rollbacks the database to a specified historical point. The state is
// supported as the rollback destination only if it's canonical state and the
// corresponding trie histories are existent. It's only supported by path-based
// database and will return an error for others.
func (db *Database) Recover(target common.Hash) error {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	return pdb.Recover(target, &trieLoader{db: db})
}

// Recoverable returns the indicator if the specified state is enabled to be
// recovered. It's only supported by path-based database and will return an
// error for others.
func (db *Database) Recoverable(root common.Hash) (bool, error) {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return false, errors.New("not supported")
	}
	return pdb.Recoverable(root), nil
}

// Disable deactivates the database and invalidates all available state layers
// as stale to prevent access to the persistent state, which is in the syncing
// stage.
//
// It's only supported by path-based database and will return an error for others.
func (db *Database) Disable() error {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	return pdb.Disable()
}

// Enable activates database and resets the state tree with the provided persistent
// state root once the state sync is finished.
func (db *Database) Enable(root common.Hash) error {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	return pdb.Enable(root)
}

// Journal commits an entire diff hierarchy to disk into a single journal entry.
// This is meant to be used during shutdown to persist the snapshot without
// flattening everything down (bad for reorgs). It's only supported by path-based
// database and will return an error for others.
func (db *Database) Journal(root common.Hash) error {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	return pdb.Journal(root)
}

// SetBufferSize sets the node buffer size to the provided value(in bytes).
// It's only supported by path-based database and will return an error for
// others.
func (db *Database) SetBufferSize(size int) error {
	pdb, ok := db.backend.(*pathdb.Database)
	if !ok {
		return errors.New("not supported")
	}
	return pdb.SetBufferSize(size)
}
