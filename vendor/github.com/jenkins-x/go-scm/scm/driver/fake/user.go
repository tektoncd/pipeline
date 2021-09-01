package fake

import (
	"context"

	"github.com/jenkins-x/go-scm/scm"
)

type userService struct {
	client *wrapper
	data   *Data
}

func (s *userService) CreateToken(context.Context, string, string) (*scm.UserToken, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *userService) DeleteToken(context.Context, int64) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *userService) Find(ctx context.Context) (*scm.User, *scm.Response, error) {
	return &s.data.CurrentUser, nil, nil
}

func (s *userService) FindEmail(ctx context.Context) (string, *scm.Response, error) {
	return s.data.CurrentUser.Email, nil, nil
}

func (s *userService) FindLogin(ctx context.Context, login string) (*scm.User, *scm.Response, error) {
	for _, user := range s.data.Users {
		if user.Login == login {
			return user, nil, nil
		}
	}
	return nil, nil, nil
}

func (s *userService) ListInvitations(context.Context) ([]*scm.Invitation, *scm.Response, error) {
	return s.data.Invitations, nil, nil
}

func (s *userService) AcceptInvitation(_ context.Context, id int64) (*scm.Response, error) {
	invitations := s.data.Invitations
	for i, invite := range invitations {
		if invite.ID == id {
			remaining := invitations[0:i]
			if i+1 < len(invitations) {
				remaining = append(remaining, invitations[i+1:]...)
			}
			s.data.Invitations = remaining
			return nil, nil
		}
	}
	return nil, scm.ErrNotSupported
}
