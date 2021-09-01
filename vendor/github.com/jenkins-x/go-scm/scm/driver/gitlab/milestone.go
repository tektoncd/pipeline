package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type milestoneService struct {
	client *wrapper
}

// isoTime represents an ISO 8601 formatted date
type isoTime time.Time

// ISO 8601 date format
const iso8601 = "2006-01-02"

// MarshalJSON implements the json.Marshaler interface
func (t isoTime) MarshalJSON() ([]byte, error) {
	if y := time.Time(t).Year(); y < 0 || y >= 10000 {
		// ISO 8901 uses 4 digits for the years
		return nil, errors.New("json: ISOTime year outside of range [0,9999]")
	}

	b := make([]byte, 0, len(iso8601)+2)
	b = append(b, '"')
	b = time.Time(t).AppendFormat(b, iso8601)
	b = append(b, '"')

	return b, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (t *isoTime) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package
	if string(data) == "null" {
		return nil
	}

	isotime, err := time.Parse(`"`+iso8601+`"`, string(data))
	*t = isoTime(isotime)

	return err
}

// EncodeValues implements the query.Encoder interface
func (t *isoTime) EncodeValues(key string, v *url.Values) error {
	if t == nil || (time.Time(*t)).IsZero() {
		return nil
	}
	v.Add(key, t.String())
	return nil
}

// String implements the Stringer interface
func (t isoTime) String() string {
	return time.Time(t).Format(iso8601)
}

type milestone struct {
	ID          int       `json:"id"`
	IID         int       `json:"iid"`
	ProjectID   int       `json:"project_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	State       string    `json:"state"`
	DueDate     isoTime   `json:"due_date"`
	StartDate   isoTime   `json:"start_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Expired     bool      `json:"expired"`
}

type milestoneInput struct {
	Title       *string  `json:"title"`
	StateEvent  *string  `json:"state_event,omitempty"`
	Description *string  `json:"description"`
	DueDate     *isoTime `json:"due_date"`
}

func (s *milestoneService) Find(ctx context.Context, repo string, id int) (*scm.Milestone, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/milestones/%d", encode(repo), id)
	out := new(milestone)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertMilestone(out), res, err
}

func (s *milestoneService) List(ctx context.Context, repo string, opts scm.MilestoneListOptions) ([]*scm.Milestone, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/milestones?%s", encode(repo), encodeMilestoneListOptions(opts))
	out := []*milestone{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertMilestoneList(out), res, err
}

func (s *milestoneService) Create(ctx context.Context, repo string, input *scm.MilestoneInput) (*scm.Milestone, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/milestones", encode(repo))
	dueDateIso := isoTime(*input.DueDate)
	in := &milestoneInput{
		Title:       &input.Title,
		Description: &input.Description,
		DueDate:     &dueDateIso,
	}
	out := new(milestone)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertMilestone(out), res, err
}

func (s *milestoneService) Delete(ctx context.Context, repo string, id int) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/milestones/%d", encode(repo), id)
	res, err := s.client.do(ctx, "DELETE", path, nil, nil)
	return res, err
}

func (s *milestoneService) Update(ctx context.Context, repo string, id int, input *scm.MilestoneInput) (*scm.Milestone, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/milestones/%d", encode(repo), id)
	in := &milestoneInput{}
	if input.Title != "" {
		in.Title = &input.Title
	}
	if input.State != "" {
		if input.State == "open" {
			activate := "activate"
			in.StateEvent = &activate
		} else {
			in.StateEvent = &input.State
		}
	}
	if input.Description != "" {
		in.Description = &input.Description
	}
	if input.DueDate != nil {
		dueDateIso := isoTime(*input.DueDate)
		in.DueDate = &dueDateIso
	}
	out := new(milestone)
	res, err := s.client.do(ctx, "PATCH", path, in, out)
	return convertMilestone(out), res, err
}

func convertMilestoneList(from []*milestone) []*scm.Milestone {
	var to []*scm.Milestone
	for _, m := range from {
		to = append(to, convertMilestone(m))
	}
	return to
}

func convertMilestone(from *milestone) *scm.Milestone {
	if from == nil || from.Title == "" {
		return nil
	}
	dueDate := time.Time(from.DueDate)
	return &scm.Milestone{
		Number:      from.ID,
		ID:          from.ID,
		Title:       from.Title,
		Description: from.Description,
		State:       from.State,
		DueDate:     &dueDate,
	}
}
