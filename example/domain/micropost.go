package domain

//+test
type Micropost struct {
	ID        string
	Author    User
	Content   string
	LikeCount uint32
	Tag       []Tag
}
