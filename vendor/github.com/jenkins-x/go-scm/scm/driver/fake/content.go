package fake

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/pkg/errors"
)

const (

	// DefaultFileWritePermissions default permissions when creating a file
	DefaultFileWritePermissions = 0o644
)

type contentService struct {
	client *wrapper
	data   *Data
}

func (c contentService) Find(_ context.Context, repo, path, ref string) (*scm.Content, *scm.Response, error) {
	f, err := c.path(repo, path, ref)
	if err != nil {
		return nil, nil, err
	}
	_, err = os.Stat(f)
	if os.IsNotExist(err) {
		return nil, &scm.Response{
			Status: 404,
		}, errors.Wrapf(err, "file %s does not exist", f)
	}
	data, err := os.ReadFile(f) // #nosec
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to read file %s", f)
	}
	return &scm.Content{
		Path: path,
		Data: data,
		Sha:  ref,
	}, nil, nil
}

func (c contentService) List(_ context.Context, repo, path, ref string) ([]*scm.FileEntry, *scm.Response, error) {
	dir, err := c.path(repo, path, ref)
	if err != nil {
		return nil, nil, err
	}
	fileNames, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to list files in directory %s", dir)
	}
	var answer []*scm.FileEntry
	for _, f := range fileNames {
		name := f.Name()
		t := "file"
		if f.IsDir() {
			t = "dir"
		}
		path := filepath.Join(dir, name)
		info, err := f.Info()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot get info for file %s: %v", name, err)
		}
		fSize := info.Size()

		answer = append(answer, &scm.FileEntry{
			Name: name,
			Path: path,
			Type: t,
			Size: int(fSize),
			Sha:  ref,
			Link: "file://" + path,
		})
	}
	return answer, nil, nil
}

func (c contentService) Create(_ context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	f, err := c.path(repo, path, "")
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(f, params.Data, DefaultFileWritePermissions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to write file %s", f)
	}
	return nil, nil
}

func (c contentService) Update(_ context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	f, err := c.path(repo, path, "")
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(f, params.Data, DefaultFileWritePermissions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to write file %s", f)
	}
	return nil, nil
}

func (c contentService) Delete(_ context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	f, err := c.path(repo, path, params.Ref)
	if err != nil {
		return nil, err
	}
	err = os.Remove(f)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to delete file %s", f)
	}
	return nil, nil
}

func (c contentService) path(repo, path, ref string) (string, error) {
	if c.data.ContentDir == "" {
		return "", errors.Errorf("no data.ContentDir configured")
	}
	if ref == "" {
		ref = "master"
	}
	repoDir := filepath.Join(c.data.ContentDir, repo)

	// lets see if there's a 'refs' folder for testing out different files in different ref/shas
	refDir := filepath.Join(repoDir, "refs", ref)
	exists, err := DirExists(refDir)
	if err != nil {
		return repoDir, errors.Wrapf(err, "failed to check if refs dir %s exists", refDir)
	}
	if exists {
		repoDir = refDir
	}
	return filepath.Join(repoDir, path), nil
}

// DirExists checks if path exists and is a directory
func DirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.IsDir(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
