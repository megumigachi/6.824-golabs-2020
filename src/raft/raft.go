package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)
import "sync/atomic"
import "../labrpc"

// import "bytes"
// import "../labgob"

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

//all roles
const (
	Leader    = 0
	Candidate = 1
	Follower  = 2
)

//
// A Go object implementing a single Raft peer.
//

const (
	ElectionTimeout=300
	HeartBeatTimeOut=100
	RpcToleranceTimeOut=100
)

type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	role     int
	lockName string
	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	currentTerm int
	voteFor     int
	//todo  log
	log []Log

	//volatile server state
	commitIndex int
	lastApplied int

	//volatile leader state
	nextIndex     []int
	matchedIndex  []int

	//other
	electionTimer *time.Timer

	//leader
	//heartBeatTimer *time.Timer
	appendEntriesTimers []*time.Timer

	//
	debugTimer *time.Timer

	lockTime time.Time

	stopSignal chan int
}


type Log struct {
	Term    int
	Index     int
	Command interface{}
}



// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.lock("getState")
	defer rf.unlock("getState")
	var term int
	var isleader bool
	// Your code here (2A).
	term=rf.currentTerm
	isleader=rf.role==Leader
	return term, isleader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

func (rf* Raft) lock(name string){
	rf.mu.Lock()
	rf.lockTime=time.Now()
	rf.lockName=name
}

func (rf* Raft) unlock(name string){
	defer rf.mu.Unlock()
	log.Printf("server id :%d,unlock from %s",rf.me,rf.lockName)
	rf.lockName=""
}




//
func (rf *Raft) generateRandomElectionTimeOut() time.Duration{
	return time.Duration(rand.Intn(150) + ElectionTimeout)*time.Millisecond
}


//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).

	return index, term, isLeader
}

//
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
	rf.stopSignal<-1
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) getLastLogIdxAndTerm() (int, int) {
	//rf.lock("getLogIndx")
	//defer rf.unlock("getLogIdx")
	log:=rf.log
	if len(log)==0 {
		return 0,0
	}
	return len(log),log[len(log)-1].Term
}

func (rf* Raft) changeRole(role int)  {
	rf.lock("changeRole")
	defer rf.unlock("changeRole")

	// change role
	rf.role=role

	// deal with timers
	if role==Follower||role==Candidate {
		rf.electionTimer.Reset(ElectionTimeout)
		for i:=0;i< len(rf.appendEntriesTimers); i++ {
			rf.appendEntriesTimers[i].Stop()
		}
	}else {
		rf.electionTimer.Stop()
		for i:=0;i<len(rf.appendEntriesTimers) ;i++  {
			rf.appendEntriesTimers[i].Reset(HeartBeatTimeOut)
		}
	}

	// set leader state

	// prepare for election
	if role ==Candidate {
		rf.currentTerm++
		rf.voteFor=rf.me
	}
}

func (rf *Raft) reportState() {
	if Debug>0{
		log.Printf("id:%d  role:%d  lockname:%s",rf.me,rf.role,rf.lockName)
	}
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	rf.electionTimer=time.NewTimer(rf.generateRandomElectionTimeOut())
	rf.appendEntriesTimers=make([]*time.Timer, len(peers))
	for i:=0;i<len(peers) ;i++  {
		rf.appendEntriesTimers[i]=time.NewTimer(HeartBeatTimeOut)
	}
	rf.changeRole(Follower)
	//rf.electionTimer.Stop()
	rf.voteFor=-1
	rf.log=make([]Log,0)

	// election listener
	go func() {
		for   {
			select {
				case <-rf.stopSignal:
					return;
				case <-rf.electionTimer.C:{
					rf.changeRole(Candidate)
					rf.startElection()
				}
			}
		}
	}()

	//append entries
	for i:=0;i< len(rf.peers);i++  {
		idx:=i
		if idx==rf.me {
			continue
		}
		go func() {
			//log.Printf("i=%d",idx)
			for   {
				select {
					case <-rf.stopSignal:
						return;
					case <-rf.appendEntriesTimers[idx].C:{
						rf.appendEntriesToFollower(idx)
					}
				}
			}
		}()
	}


	rf.debugTimer=time.NewTimer(500*time.Microsecond)
	//debug : watching lockName
	go func() {
		for !rf.killed() {
			time.Sleep(time.Second * 2)
			fmt.Println(fmt.Sprintf("rf who has lock:%s, time:%v", rf.lockName, time.Now().Sub(rf.lockTime)))
			fmt.Println(fmt.Sprintf("server id:%d	,role: %d,term :%d ,votedFor:%d",rf.me,rf.role,rf.currentTerm,rf.voteFor))
		}

	}()

	// Your initialization code here (2A, 2B, 2C).


	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	return rf
}
