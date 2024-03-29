package raft

import (
	"time"
)

type InstallSnapshotArgs struct {
	Term             int
	LeaderId         int
	LastIncludedIdx  int
	LastIncludedTerm int
	Data             []byte
}

type InstallSnapshotReply struct {
	Term int
}

//raft 需要在节点中传输snapshot 所以虽然snapshot 理论上归server管理，但是
//还是要和raft关联。
//再加上persister没有单独存储snapshot的接口，所以存储snapshot放到raft里
func (rf *Raft)SaveSnapshotAndState(lastLogIdx int, snapshot []byte)  {
	rf.lock("save_snapshot")
	defer rf.unlock("save_snapshot")

	DPrintf("[rf %d][SaveSnapshotAndState][lastidx %d][snapshot size %d]",rf.me,lastLogIdx, len(snapshot))
	// 为什么会传一个更小的快照？
	// 相当于已提交的日志被撤销了，怎么都不可能
	if rf.lastSnapshotIdx>=lastLogIdx {
		return
	}

	//默认新参数更大
	logIdx,term:=rf.getLogIdxAndTermByRealIdx(lastLogIdx)
	rf.lastSnapshotIdx,rf.lastSnapshotTerm=lastLogIdx,term
	newLog:=make([]Log,0)
	newLog=append(newLog, Log{
		Term:    term,
		Index:   0,
		Command: nil,
	})
	rf.log=append(newLog,rf.log[logIdx+1:]...)
	state:=rf.generatePersistData()
	rf.persister.SaveStateAndSnapshot(state,snapshot)
}


/*
作为append entry 的代替，对于idx的server 发送snapshot
如果成功，重置timer，更新属性
 */
func (rf *Raft) sendSnapshot (idx int) {
	rf.lock("sendSnapshot")
	args:=&InstallSnapshotArgs{
		Term:             rf.currentTerm,
		LeaderId:         rf.me,
		LastIncludedIdx:  rf.lastSnapshotIdx,
		LastIncludedTerm: rf.lastSnapshotTerm,
		Data:             rf.persister.ReadSnapshot(),
	}
	reply:=&InstallSnapshotReply{
		Term:-1,
	}
	boolChan:=make(chan bool,1)
	rpcTimer:=time.NewTimer(RpcToleranceTimeOut*time.Millisecond)
	rf.appendEntriesTimers[idx].Reset(HeartBeatTimeOut*time.Millisecond)
	DPrintf("[rf %d][begin install snapshot][target id %d][last idx %d][snapshot size %d]",rf.me,idx,rf.lastSnapshotIdx,len(args.Data))
	rf.unlock("sendSnapshot")
	go func() {
		ok:=rf.peers[idx].Call("Raft.InstallSnapshot", args, reply)
		boolChan<-ok
	}()

	select {
		case <-rpcTimer.C:{
			DPrintf("[rf %d][install snapshot failed][target %d] rpc timeout ",rf.me,idx)
			return
		}
		case <-rf.stopSignal:
			return
		case ok:=<-boolChan:{
			rf.lock("sendSnapshot_2")
			defer rf.unlock("sendSnapshot_2")
			if rf.role!=Leader {
				return
			}
			if ok {
				term:=reply.Term
				if term>rf.currentTerm {
					DPrintf("[rf %d][install snapshot failed]term less than target,turn to follower",rf.me)
					rf.currentTerm=term
					rf.persist()
					rf.changeRole(Follower)
				}else {
					DPrintf("[rf %d][install snapshot success][target id %d]",rf.me,idx)
					//默认更新成功
					rf.nextIndex[idx]=args.LastIncludedIdx+1
					rf.matchedIndex[idx]=args.LastIncludedIdx
					rf.updateCommitIndex()
				}
			}else {
				DPrintf("[rf %d][install snapshot failed][target %d] network failed ",rf.me,idx)
				rf.appendEntriesTimers[idx].Reset(10*time.Millisecond)
			}
		}
	}

}


func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	/*什么情况会出现 arg.index<server.lastindex
	此时 leader 的next< arg.index< server
	感觉怎么都举不出例子造成这种情况
	非可信网络下一切皆有可能,比如leader append entry传了一堆数据，回传的确认数据没有收到
	然后此时server进行了一轮快照，leader没有，leader 按照上次没有更新的nextIndex[server]
	来为server发送快照，就会出现一个更早的快照。
	那该怎么处理呢？首先不能应用，状态机和是绝对不可能回退的，否则违背了一致性。只能丢弃
	但是install snapshot rpc是没有success的，这会造成什么后果？
	实际上leader会把matched index 和next index都调整到该snapshotindex的位置，虽然小于实际上应该的
	matched index，但不会造成逻辑上的错误
	*/
	rf.lock("install_snapshot")
	defer rf.unlock("install_snapshot")

	leaderId:=args.LeaderId


	reply.Term=rf.currentTerm

	if args.Term<rf.currentTerm {
		return
	}else if args.Term>rf.currentTerm{
		rf.currentTerm=args.Term
		rf.persist()
		rf.changeRole(Follower)
		reply.Term=rf.currentTerm
	}

	//if args.LastIncludedIdx<rf.lastSnapshotIdx {
	//	log.Panicf("[install snapshot %d][args.index<lastSnapshotIndex]",rf.me)
	//	return
	//}
	if args.LastIncludedIdx<=rf.lastSnapshotIdx {
		return
	}
	DPrintf("[rf %d][accept install snapshot][from %d][lastidx %d][received Data len %d]",rf.me,leaderId,args.LastIncludedIdx,len(args.Data))

	rf.resetElectionTimer()
	beginIdx:=rf.getLogIdxByRealIdx(args.LastIncludedIdx)
	newLog:=make([]Log,0)
	newLog=append(newLog, Log{
		Term:    args.LastIncludedTerm, //这里把lastSnapshotTerm填充进去，实际上只是为了一致性检查方便点
		Index:   0,
		Command: nil,
	})
	//argidx - last
	if beginIdx<0||beginIdx>= len(rf.log) {
		rf.log=newLog
	}else {
		rf.log=append(newLog, rf.log[beginIdx+1:]...)
	}
	rf.lastSnapshotIdx=args.LastIncludedIdx
	rf.lastSnapshotTerm=args.LastIncludedTerm
	if rf.commitIndex<rf.lastSnapshotIdx {
		rf.commitIndex=rf.lastSnapshotIdx
	}
	rf.persister.SaveStateAndSnapshot(rf.generatePersistData(),args.Data)
	rf.applyTimer.Reset(0)
}