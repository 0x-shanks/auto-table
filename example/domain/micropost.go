package domain

//+test
type Micropost struct {
	ID       string
	AuthorID string `test:"fk:User.ID"`
	//Author    User
	Content   string
	LikeCount uint32
	Tag       []Tag
}
