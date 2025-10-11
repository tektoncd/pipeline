// Copyright 2023 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"time"
)

// Package represents a package
type Package struct {
	// the package's id
	ID int64 `json:"id"`
	// the package's owner
	Owner User `json:"owner"`
	// the repo this package belongs to (if any)
	Repository *Repository `json:"repository"`
	// the package's creator
	Creator User `json:"creator"`
	// the type of package:
	Type string `json:"type"`
	// the name of the package
	Name string `json:"name"`
	// the version of the package
	Version string `json:"version"`
	// the date the package was uploaded
	CreatedAt time.Time `json:"created_at"`
}

// PackageFile represents a file from a package
type PackageFile struct {
	// the file's ID
	ID int64 `json:"id"`
	// the size of the file in bytes
	Size int64 `json:"size"`
	// the name of the file
	Name string `json:"name"`
	// the md5 hash of the file
	MD5 string `json:"md5"`
	// the sha1 hash of the file
	SHA1 string `json:"sha1"`
	// the sha256 hash of the file
	SHA256 string `json:"sha256"`
	// the sha512 hash of the file
	SHA512 string `json:"sha512"`
}

// ListPackagesOptions options for listing packages
type ListPackagesOptions struct {
	ListOptions
}

// ListPackages lists all the packages owned by a given owner (user, organisation)
func (c *Client) ListPackages(owner string, opt ListPackagesOptions) ([]*Package, *Response, error) {
	if err := escapeValidatePathSegments(&owner); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	packages := make([]*Package, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/packages/%s?%s", owner, opt.getURLQuery().Encode()), nil, nil, &packages)
	return packages, resp, err
}

// GetPackage gets the details of a specific package version
func (c *Client) GetPackage(owner, packageType, name, version string) (*Package, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &packageType, &name, &version); err != nil {
		return nil, nil, err
	}
	foundPackage := new(Package)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/packages/%s/%s/%s/%s", owner, packageType, name, version), nil, nil, foundPackage)
	return foundPackage, resp, err
}

// DeletePackage deletes a specific package version
func (c *Client) DeletePackage(owner, packageType, name, version string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &packageType, &name, &version); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/packages/%s/%s/%s/%s", owner, packageType, name, version), nil, nil)
}

// ListPackageFiles lists the files within a package
func (c *Client) ListPackageFiles(owner, packageType, name, version string) ([]*PackageFile, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &packageType, &name, &version); err != nil {
		return nil, nil, err
	}
	packageFiles := make([]*PackageFile, 0)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/packages/%s/%s/%s/%s/files", owner, packageType, name, version), nil, nil, &packageFiles)
	return packageFiles, resp, err
}
