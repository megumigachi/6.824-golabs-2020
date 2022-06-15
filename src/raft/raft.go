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
	"../labgob"
	"bytes"
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
	ApplyTimeOut=50
)

type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	applyCh chan ApplyMsg

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

	//except leader
	electionTimer *time.Timer

	//leader
	//heartBeatTimer *time.Timer
	appendEntriesTimers []*time.Timer

	//for all servers , apply log entries periodically
	applyTimer *time.Timer


	//for debug
	lockTime time.Time
	startTime time.Time


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



func (rf* Raft) lock(name string){
	rf.mu.Lock()
	rf.lockTime=time.Now()
	rf.lockName=name
}

func (rf* Raft) unlock(name string){
	defer rf.mu.Unlock()
	if time.Now().Sub(rf.lockTime).Milliseconds()>5 {
		DPrintf("lock time too long: server id :%d,unlock from %s, lock time %v",rf.me,rf.lockName,time.Now().Sub(rf.lockTime))
	}
	rf.lockName=""
}




//
func (rf *Raft) generateRandomElectionTimeOut() time.Duration{
	duration:=time.Duration(rand.Intn(300) + ElectionTimeout)*time.Millisecond
	return duration
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
	rf.lock("start")
	index,_:=rf.getLastLogIdxAndTerm()
	term := rf.currentTerm
	isLeader := rf.role==Leader

	// Your code here (2B).
	if rf.role==Leader {
		newLog:=Log{}
		newLog.Term=rf.currentTerm
		newLog.Command=command
		rf.log=append(rf.log, newLog)
		rf.persist()
		DPrintf("start agreement: rf.id is%d,rf.log length is %d,return index:%d , command is %v , time is %v", rf.me,len(rf.log)-1,index+1,command, time.Now().Sub(rf.startTime))
		//对于每一个server立即发送一条append entry 请求
		for i:=0;i<rf.me;i++ {
			if rf.me==i{
				continue
			}
			t:=rf.appendEntriesTimers[i]
			t.Stop()
			t.Reset(0)
		}

	}

	rf.unlock("start")
	return index+1, term, isLeader
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
	close(rf.stopSignal)
	log.Println(fmt.Sprintf("killed: server id:%d,log length:%d,commitIdx:%d",rf.me,len(rf.log)-1,rf.commitIndex))

}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) getLastLogIdxAndTerm() (int, int) {

	log:=rf.log
	//if len(log)==0 {
	//	return -1,0
	//}
	return len(log)-1,log[len(log)-1].Term
}

func (rf* Raft) changeRole(role int)  {
	//DPrintf("change role: serverid: %d,changerole %d, time:%v \n",rf.me,role,rf.getTimeLine())

	// prepare for election
	if role ==Candidate {
		rf.currentTerm++
		rf.voteFor=rf.me
		rf.persist()
	}


	if role==rf.role {
		return
	}

	//todo: init leader properties
	if role==Leader {
		rf.initLeaderProperties()
	}


	// change role
	rf.role=role

	// deal with timers
	if role==Candidate||role==Follower {
		rf.resetElectionTimer()
		for i:=0;i< len(rf.appendEntriesTimers); i++ {
			if rf.me==i {
				continue
			}
			rf.appendEntriesTimers[i].Stop()
		}
	}

	if role==Leader{
		rf.electionTimer.Stop()
		for i:=0;i<len(rf.appendEntriesTimers) ;i++  {
			if rf.me==i {
				continue
			}
			rf.appendEntriesTimers[i].Reset(HeartBeatTimeOut*time.Millisecond)
		}
	}

	// set leader state


}



func (rf *Raft) getTimeLine() time.Duration {
	return time.Now().Sub(rf.startTime)
}

func (rf *Raft) initLeaderProperties() {
	rf.nextIndex=make([]int, len(rf.peers))
	rf.matchedIndex=make([]int, len(rf.peers))

	idx,_:=rf.getLastLogIdxAndTerm()
	for i:=0;i< len(rf.nextIndex);i++  {
		rf.nextIndex[i]=idx+1
		rf.matchedIndex[i]=0
	}
}

func (rf *Raft) applyLogs() {
	rf.lock("apply logs")
	rf.applyTimer.Reset(ApplyTimeOut*time.Millisecond)
	if rf.lastApplied>rf.commitIndex {
		panic("apply an uncommitted index ")
	}else if rf.lastApplied==rf.commitIndex{
		rf.unlock("apply logs")
		return
	}else {
		DPrintf("start applying logs, server id:%d ,from:%d , to %d, time %v",rf.me,rf.lastApplied+1,rf.commitIndex,time.Now().Sub(rf.startTime))
		msgs:=make([]ApplyMsg,0)
		for rf.lastApplied<rf.commitIndex  {
			rf.lastApplied++
			index:=rf.lastApplied
			command:=rf.log[index].Command
			applymsg := ApplyMsg{
				true,
				command,
				index,
			}
			msgs=append(msgs,applymsg)
			//DPrintf("server id:%d ,apply log id:%d",rf.me,index)
		}
		rf.unlock("apply logs")
		go func() {
			for _,item:=range msgs {
				rf.applyCh<-item
			}
		}()

	}
}


//not concurrent-safe
func (rf *Raft) persist() {
	// Your code here (2C).
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.voteFor)
	e.Encode(rf.log)
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}

func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	rf.lock("readPersist")
	defer rf.unlock("readPersist")
	// Your code here (2C).
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)

	d.Decode(&rf.currentTerm)
	d.Decode(&rf.voteFor)
	d.Decode(&rf.log)

	DPrintf("reading persist, id is %d, currentTerm is %d, votefor:%d, log length:%d\n" ,rf.me,rf.currentTerm,rf.voteFor, len(rf.log)-1)
}

func (rf *Raft) printState() {
	rf.lock("printState")
	defer rf.unlock("printState")
	DPrintf("printState: id=%d, term=%d, role=%d, log length:%d ,last log term:%d, commitIndex:%d\n", rf.me,rf.currentTerm,rf.role, len(rf.log)-1,rf.log[len(rf.log)-1].Term,rf.commitIndex)
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
	//labgob.Register(Command{})
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.applyCh=applyCh
	rf.startTime=time.Now()

	rf.role=-1
	rf.stopSignal=make(chan int)


	//todo persist
	if persister.RaftStateSize()==0 {
		//DPrintf("init... id:%d\n",rf.me)
		rf.currentTerm=0
		rf.voteFor=-1
		rf.log=make([]Log,0)
		//初始化填充
		if len(rf.log)==0 {
			rf.log=append(rf.log,Log{
				Term:    0,
				Index:   0,
				Command: nil,
			})
		}
		rf.persist()
	}else {
		rf.readPersist(persister.ReadRaftState())
	}




	//volatile properties
	rf.commitIndex=0
	rf.lastApplied=0


	rf.electionTimer=time.NewTimer(time.Duration(me+ElectionTimeout)*time.Millisecond)
	rf.appendEntriesTimers=make([]*time.Timer, len(peers))
	for i:=0;i<len(peers) ;i++  {
		if i==rf.me {
			continue
		}
		rf.appendEntriesTimers[i]=time.NewTimer(HeartBeatTimeOut*time.Millisecond)
	}
	rf.applyTimer=time.NewTimer(ApplyTimeOut*time.Millisecond)

	//todo:这里需要修改，因为在重启的时候可能会重新读取某些属性（如votefor）而不会按照这个逻辑
	rf.changeRole(Follower)
	//用来人造bug
	//rf.electionTimer.Reset(time.Duration(me+ElectionTimeout)*time.Millisecond)
	//rf.electionTimer.Stop()




	// election listener
	//now:=time.Now()
	go func() {
		for   {
			select {
				case <-rf.stopSignal:
					return;
				case <-rf.electionTimer.C:{
					//fmt.Printf("server id:%d start election, currentTerm: %d,time :%v \n",rf.me,rf.currentTerm,time.Now().Sub(now))

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
			//DPrintf("i=%d",idx)
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

	//applying message
	go func() {
		for  {
			select {
				case <-rf.stopSignal:
					return
				case <-rf.applyTimer.C:
					rf.applyLogs()
			}
		}
	}()

	//debug : watching lockName
	//go func() {
	//	for !rf.killed() {
	//		time.Sleep(time.Millisecond * 300)
	//		//log.Println(fmt.Sprintf("server id:%d,lock:%s, time:%v", rf.me,rf.lockName, time.Now().Sub(rf.startTime)))
	//		//log.Println(fmt.Sprintf("server id:%d	,role: %d,term :%d ,votedFor:%d",rf.me,rf.role,rf.currentTerm,rf.voteFor))
	//		log.Println(fmt.Sprintf("server id:%d,log length:%d,commitIdx:%d",rf.me,len(rf.log),rf.commitIndex))
	//
	//	}
	//
	//}()
	// Your initialization code here (2A, 2B, 2C).
	//log.Println(fmt.Sprintf("server id:%d	,role: %d,term :%d ,votedFor:%d",rf.me,rf.role,rf.currentTerm,rf.voteFor))
	// initialize from state persisted before a crash

	return rf
}
