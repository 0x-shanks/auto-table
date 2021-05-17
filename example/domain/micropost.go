package domain

//+test
type Micropost struct {
	ID        string
	AuthorID  string `test:"User"`
	Content   string
	LikeCount uint32
	Tag       []Tag
}
