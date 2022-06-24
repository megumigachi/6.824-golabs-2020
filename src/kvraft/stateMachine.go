package kvraft

type StateMachine struct {
	Data map[string]string
}

func (sm *StateMachine)Get(key string)(string,string)  {
	if v,ok:=sm.Data[key];ok {
		return v,OK
	}else{
		return "",ErrNoKey
	}
}

func (sm *StateMachine)Put(key string,value string)  {
	sm.Data[key]=value
}

func(sm* StateMachine)Append(key string,value string){
	if _,ok:=sm.Data[key];ok {
		sm.Data[key]+=value
	}else{
		sm.Data[key]=value
	}
}

