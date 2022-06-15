package kvraft

type StateMachine struct {
	data map[string]string
}

func (sm *StateMachine)Get(key string)(string,string)  {
	if v,ok:=sm.data[key];ok {
		return v,OK
	}else{
		return "",ErrNoKey
	}
}

func (sm *StateMachine)Put(key string,value string)  {
	sm.data[key]=value
}

func(sm* StateMachine)Append(key string,value string){
	if _,ok:=sm.data[key];ok {
		sm.data[key]+=value
	}else{
		sm.data[key]=value
	}
}