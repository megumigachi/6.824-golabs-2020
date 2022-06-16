package kvraft

import (
	"../labgob"
	"../labrpc"
	"../raft"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const RequestTimeOut=100*time.Millisecond

const Debug = 1

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}


//大概是用于raft中的command
type Op struct {
	Operation string
	Key string
	Value string
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
}

//对于一个客户端来说，执行完一条命令才会执行下一条
//【执行完】指一条命令的结果被客户端认可
// 也就是一旦发出了后面的命令，前面的命令理论上不可能重发
type ResponseRecord struct {
	CommandId int
	Response ResponseMessage
}

type KVServer struct {
	mu      sync.RWMutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	startTime time.Time

	lockName string

	stateMachine StateMachine

	stopch chan int	//用于传递kill信号

	ResponseChans map[int]chan ResponseMessage	//对应每个log index的channel与applych联通
	resultMap	 map[int64] ResponseRecord	//记载最新的response
	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	//start an agreement
	op:=OP_GET
	key:=args.Key

	operation:=Op{
		Operation: op,
		Key:       key,
		Value:     "",
	}
	//todo:判断重复的请求
	command:=Command{
		Op:        operation,
		ClientId:  args.ClientId,
		CommandId: args.CommandId,
	}

	//start 本身持有锁,这里应该不需要？
	index,_,isLeader:=kv.rf.Start(command)
	if!isLeader{
		reply.Err=ErrWrongLeader
		return
	}

	kv.lock("generateResponse")
	ch:=kv.generateResponseChan(index)
	kv.unlock()

	select {
		case responseMsg:=<-ch:{
			reply.Err=responseMsg.Err
			reply.Value=responseMsg.Value
		}
		case <-time.After(reRequestTimeOut):{
			reply.Err=ErrTimeOut
			break
		}
	}

	go func() {
		kv.lock("deleteResponseChan")
		delete(kv.ResponseChans,index)
		kv.unlock()
	}()


}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	//start an agreement
	op:=args.Op
	key:=args.Key
	value:=args.Value

	operation:=Op{
		Operation: op,
		Key:       key,
		Value:     value,
	}
	//todo:判断重复的请求
	command:=Command{
		Op:        operation,
		ClientId:  args.ClientId,
		CommandId: args.CommandId,
	}
	DPrintf("put append command is %v server id %v time:%v",command,kv.me,time.Now().Sub(kv.startTime))

	//start 本身持有锁,这里应该不需要？
	index,_,isLeader:=kv.rf.Start(command)
	DPrintf("server : %d ,isLeader %v , time :%v ",kv.me,isLeader,time.Now().Sub(kv.startTime))

	if!isLeader{
		reply.Err=ErrWrongLeader
		return
	}

	kv.lock("generateResponse")
	ch:=kv.generateResponseChan(index)
	kv.unlock()

	select {
		case responseMsg:=<-ch:{
			reply.Err=responseMsg.Err
		}
		case <-time.After(reRequestTimeOut):{
			reply.Err=ErrTimeOut
			break
		}
	}

	go func() {
		kv.lock("deleteResponseChan")
		kv.cleanAndDeleteRChan(index)
		kv.unlock()
	}()
}

//
// the tester calls Kill() when a KVServer instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead (without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
//
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	//close(kv.stopch)
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

func (kv *KVServer) generateResponseChan(i int) chan ResponseMessage {
	DPrintf("generateResponseChan  server id:%d, index:%d",kv.me,i)
	//channel需要清理，所以同一个index理当没有channel
	if _,ok:=kv.ResponseChans[i];ok {
		panic("error: an invalid channel ")
	}else {
		//这里怎么处理呢
		kv.ResponseChans[i]=make(chan ResponseMessage,1)
		return kv.ResponseChans[i]
	}
}

func (kv* KVServer) lock(name string){
	kv.mu.Lock()
	kv.lockName=name
}

func (kv* KVServer) unlock(){
	defer kv.mu.Unlock()
	kv.lockName=""
}

//func (kv *KVServer)  {
//
//}

//无锁方法，调用外部需要加锁
func (kv *KVServer) executeOperation(cmd Command) ResponseMessage{
	//kv.lock("executeOp")
	//defer kv.unlock()
	commandId:=cmd.CommandId
	clientId:=cmd.ClientId
	op:=cmd.Op
	//already executed
	if v,ok:=kv.resultMap[clientId];ok {
		if v.CommandId==commandId {
			return v.Response
		}else if v.CommandId>commandId{
			log.Panicf("a previous command ? command id:%v ,stored cmd id:%v",commandId,v.CommandId)
		}
	}

	responseMsg:=ResponseMessage{
		Err:   OK,
		Value: "",
	}

	if op.Operation==OP_GET {
		v,err:=kv.stateMachine.Get(op.Key)
		responseMsg.Err= Err(err)
		responseMsg.Value=v
	}else if op.Operation==OP_Put {
		kv.stateMachine.Put(op.Key,op.Value)
	}else if op.Operation==OP_Append {
		kv.stateMachine.Append(op.Key,op.Value)
	}else {

	}

	DPrintf("execute op save result map clientId:%d , commandId: %d, response message %v",clientId,commandId,responseMsg)
	kv.resultMap[clientId]=ResponseRecord{
		CommandId: commandId,
		Response:  responseMsg,
	}
	return responseMsg
}

func (kv *KVServer) cleanAndDeleteRChan(i int) {
	close(kv.ResponseChans[i])
	delete(kv.ResponseChans,i)
}

func (kv *KVServer) readApplych() {

}

//
// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
//
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})
	labgob.Register(Command{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.
	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)
	// You may need initialization code here.
	kv.stopch=make(chan int)
	kv.stateMachine=StateMachine{
		data:make(map[string]string),
	}
	kv.ResponseChans=make(map[int]chan ResponseMessage)
	kv.resultMap=make(map[int64]ResponseRecord)

	kv.startTime=time.Now()
	//从applyCh中读取数据
	go func() {
		for m:=range kv.applyCh {
			idx:=m.CommandIndex
			command:=m.Command
			valid:=m.CommandValid
			DPrintf("server id :%d , server apply message idx:%v, command:%v",kv.me,idx,command)
			if valid {
				//apply command to state machine and then tell notify channel if possible
				cmd:=command.(Command)
				kv.lock("apply operation")
				msg:=kv.executeOperation(cmd)
				if v,ok:=kv.ResponseChans[idx];ok{
					v<-msg
				}else {
					//maybe not a leader
					//DPrintf("response has been timeout\n")
				}
				kv.unlock()
			}else {
				//todo:invalid command?
			}
		}
	}()


	//check deadlock
	//go func() {
	//	for  {
	//		time.Sleep(2*time.Second)
	//		DPrintf(" server id %d , lockName %v", kv.me, kv.lockName)
	//	}
	//
	//}()

	return kv
}
