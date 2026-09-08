package application

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
}

type Captcha struct {
	ID    string
	Image string
}

type CurrentUser struct {
	UserID      int64
	Username    string
	Nickname    string
	Avatar      string
	Roles       []string
	Permissions []string
}

type Authorization struct {
	Roles       []string
	Permissions []string
	System      bool
}

type AccountScope struct {
	All           bool
	SelfID        int64
	DepartmentIDs []int64
}
