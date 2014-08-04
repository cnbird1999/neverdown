package neverdown

import (
	"io"
	"net"
	"os"
	"fmt"
	"path"
	"time"
	"log"
	"strconv"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-mdb"
)

// Command is the command type as stored in the raft log
type Command uint8

const (
	addCmd Command = iota
	rmCmd

	uuidOffset  = 2
	entryOffset = 18

	logSchemaVersion  = 0x0
	snapSchemaVersion = 0x0
)

// FSM wraps the Storage instance in the raft.FSM interface, allowing raft to apply commands.
type FSM struct {
	store *Store
}

func (fsm *FSM) Apply(l *raft.Log) interface{} {
	//log.Printf("Got log %+v", l)
	//log.Printf("%v", string(l.Data))
	return fsm.store.ExecCommand(l.Data)
}

// Snapshot creates a raft snapshot for fast restore.
func (fsm *FSM) Snapshot() (raft.FSMSnapshot, error) {
	data, err := fsm.store.JSON()
	if err != nil {
		return nil, err
	}
	snapshot := &Snapshot{
		Data: data,
	}
	return snapshot, nil
}

// Restore from a raft snapshot
func (fsm *FSM) Restore(snap io.ReadCloser) error {
	defer snap.Close()
	return fsm.store.FromJSON(snap)
}

type Snapshot struct {
	Data []byte

}

// Persist writes a snapshot to a file. We just serialize all active entries.
func (s *Snapshot) Persist(sink raft.SnapshotSink) error {
	_, err := sink.Write(s.Data)
	if err != nil {
		sink.Cancel()
		return err
	}
	return sink.Close()
}

// Release cleans up a snapshot. We don't need to do anything.
func (s *Snapshot) Release() {
}

// Raft encapsulates the raft specific logic for startup and shutdown.
type Raft struct {
	Addr net.Addr
	Store *Store
	transport *raft.NetworkTransport
	mdb       *raftmdb.MDBStore
	raft      *raft.Raft
	peerStore *raft.JSONPeers
	fsm       *FSM
	//leader bool
}

// NewRaft initialize raft.
func NewRaft(prefix, addr string, peers []string) (r *Raft, err error) {
	r = new(Raft)

	config := raft.DefaultConfig()
	config.EnableSingleNode = true
	//var logOutput *os.File
	//logFile := path.Join(".", "raft.log")
	//logOutput, err = os.OpenFile(logFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	//if err != nil {
	//	log.Fatalf("Could not open raft log file: ", err)
	//}
	//config.LogOutput = logOutput

	raftDir := path.Join(".", prefix+"_raft")
	err = os.MkdirAll(raftDir, 0755)
	if err != nil {
		log.Fatalf("Could not create raft storage dir: ", err)
	}

	fss, err := raft.NewFileSnapshotStore(raftDir, 1, nil)
	if err != nil {
		panic(fmt.Errorf("Could not initialize raft snapshot store: ", err))
		return
	}

	peersAddr := []net.Addr{}
	for _, peer := range peers {
		peerAddr, err := net.ResolveTCPAddr("tcp", peer)
		if err != nil {
			panic(fmt.Errorf("Could not ResolveTCPAddr: ", err))
			return nil, err
		}
		peersAddr = append(peersAddr, peerAddr)
	}

	a, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		panic(fmt.Errorf("Could not lookup raft advertise address: ", err))
		return
	}
	r.Addr = a

	r.transport, err = raft.NewTCPTransport(addr, a, 3, 10*time.Second, nil)
	if err != nil {
		panic(fmt.Errorf("Could not create raft transport: ", err))
		return
	}

	peerStore := raft.NewJSONPeers(raftDir, r.transport)
	if err := peerStore.SetPeers(peersAddr); err != nil {
		panic(fmt.Errorf("Could not set peers: %v", err))
		return nil, err
	}
	r.peerStore = peerStore

	single := true
	if !single {
		var peers []net.Addr
		peers, err = peerStore.Peers()
		if err != nil {
			return
		}

		for _, peerStr := range []string{} {
			peer, err := net.ResolveTCPAddr("tcp", peerStr)
			if err != nil {
				log.Fatalf("Bad peer:", err)
			}

			if !raft.PeerContained(peers, peer) {
				peerStore.SetPeers(raft.AddUniquePeer(peers, peer))
			}
		}
	}
	r.mdb, err = raftmdb.NewMDBStore(raftDir)
	if err != nil {
		panic(fmt.Errorf("Could not create raft store:", err))
		return
	}
	r.Store = NewStore()
	r.fsm = &FSM{
		store: r.Store,
	}

	r.raft, err = raft.NewRaft(config, r.fsm, r.mdb, r.mdb, fss, peerStore, r.transport)
	if err != nil {
		panic(fmt.Errorf("Could not initialize raft: ", err))
		return
	}
	return
}

// ResolveAPIAddr return the API address for the given raft node.
func ResolveAPIAddr(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	tcpAddr, _ := net.ResolveTCPAddr("tcp", addr.String())
	ip := ""
	if tcpAddr.IP != nil {
		ip = tcpAddr.IP.String()
	}
	return ip+":"+strconv.Itoa(tcpAddr.Port-10)
}

// Peers returns the address of every nodes in the raft cluster.
func (r *Raft) Peers() ([]net.Addr, error) {
	return r.peerStore.Peers()
}

// PeersAPI returns the HTTP JSON API endpoints of every nodes in the raft cluster.
func (r *Raft) PeersAPI() ([]string) {
	fmt.Printf("PeersAPI")
	addrs, _ := r.Peers()
	peers := []string{}
	leaderAddr := ResolveAPIAddr(r.Leader())
	for _, addr := range addrs {
		apiAddr := ResolveAPIAddr(addr)
		if apiAddr != leaderAddr {
			peers = append(peers, apiAddr)
		}
	}
	return peers
}

// Leader returns the address of raft leader.
func (r *Raft) Leader() net.Addr {
	return r.raft.Leader()
}

// IsLeader returns true if the current node is the raft leader.
//func (r *Raft) IsLeader() bool {
//	return r.leader
//}

// Close cleanly shutsdown the raft instance.
func (r *Raft) Close() {
	r.transport.Close()
	if err := r.raft.Shutdown().Error(); err != nil {
		panic(fmt.Errorf("Error shutting down raft: ", err))
	}
	r.mdb.Close()
}

func (r *Raft) ExecCommand(msg []byte) error {
	future := r.raft.Apply(msg, 30*time.Second)
	return future.Error()
}

// Sync the FSM
func (r *Raft) Sync() error {
	return r.raft.Barrier(0).Error()
}

// LeaderCh just wraps the raft LeaderCh call
func (r *Raft) LeaderCh() <-chan bool {
	return r.raft.LeaderCh()
}
