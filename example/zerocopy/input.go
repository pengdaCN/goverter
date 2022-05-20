//go:generate goverter $GOFILE
package zerocopy

// goverter:converter
type Converter interface {
	// goverter:map CreatedAt RegisterAt
	// goverter:zeroCopy
	InnerUserToAccount(in *InnerUser) *Account
}

type InnerUser struct {
	Id         int64
	CreatedAt  int64
	UpdatedAt  int64
	Nickname   string
	Passwd     string
	GroupId    int64
	RuleId     int64
	Level      int64
	GrowNumber int64
}

type SimpleUser struct {
	Id       int64
	Nickname string
	Level    int64
}

type InnerGroup struct {
	Id        int64
	CreatedAt int64
	UpdatedAt int64
	Name      string
}

type SimpleGroup struct {
	Id   int64
	Name string
}

type Account struct {
	Id         int64
	RegisterAt int64
	Nickname   string
	Passwd     string
	GroupId    int64
	RuleId     int64
	Level      int64
	GrowNumber int64
}
