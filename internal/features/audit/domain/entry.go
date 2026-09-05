package domain

import "time"

type Entry struct {
	ID            int64
	Module        string
	ActionType    string
	Title         string
	Content       string
	OperatorID    int64
	OperatorName  string
	RequestURI    string
	RequestMethod string
	IP            string
	Region        string
	Device        string
	Browser       string
	OS            string
	Status        int
	ExecutionTime int64
	ErrorMessage  string
	CreateTime    time.Time
}

type DailyCount struct {
	Date       string
	Operations int64
	Operators  int64
}

type Counts struct {
	TodayOperations     int64
	YesterdayOperations int64
	TotalOperations     int64
	TodayOperators      int64
	YesterdayOperators  int64
	TotalOperators      int64
}
