package kvraft

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongLeader = "ErrWrongLeader"
	ErrTimeOut	   = "ErrTimeOut"
)

const (
	OP_Put="Put"
	OP_GET="Get"
	OP_Append="Append"
)

type Err string

// Put or Append
type PutAppendArgs struct {
	Key   string
	Value string
	Op    string // "Put" or "Append"
	// You'll have to add definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	ClientId int64
	CommandId int
}

type PutAppendReply struct {
	Err Err
}

type GetArgs struct {
	Key string
	// You'll have to add definitions here.
	ClientId int64
	CommandId int
}

type GetReply struct {
	Err   Err
	Value string
}

type ResponseMessage struct {
	ClientId int64
	CommandId int
	Err   Err
	Value string
}

type Command struct {
	Op Op
	ClientId int64
	CommandId int
}