package fake

import (
	"context"

	"github.com/jenkins-x/go-scm/scm"
)

type userService struct {
	client *wrapper
	data   *Data
}

func (u *userService) Find(ctx context.Context) (*scm.User, *scm.Response, error) {
	return &u.data.CurrentUser, nil, nil
}

func (u *userService) FindEmail(ctx context.Context) (string, *scm.Response, error) {
	return u.data.CurrentUser.Email, nil, nil
}

func (u *userService) FindLogin(ctx context.Context, login string) (*scm.User, *scm.Response, error) {
	for _, user := range u.data.Users {
		if user.Login == login {
			return user, nil, nil
		}
	}
	return nil, nil, nil
}
