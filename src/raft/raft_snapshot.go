package raft

type InstallSnapshotArgs struct {
	Term int
	LeaderId int
	LastIncludedIdx int
	LastIncludedTerm int
	data []byte
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

func (rf *Raft) sendSnapshot (idx int) {
	rf.lock("sendSnapshot")
	defer rf.unlock("sendSnapshot")
	args:=&InstallSnapshotArgs{
		Term:             0,
		LeaderId:         0,
		LastIncludedIdx:  0,
		LastIncludedTerm: 0,
		data:             nil,
	}
	reply:=&InstallSnapshotReply{
		Term:0,
	}
	ok:=rf.peers[idx].Call("Raft.InstallSnapshot", args, reply)

}


func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {

}