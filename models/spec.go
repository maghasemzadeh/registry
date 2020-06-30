// Copyright 2020 Google LLC. All Rights Reserved.

package models

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	rpc "apigov.dev/registry/rpc"
	ptypes "github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"
)

// SpecEntityName is used to represent specs in the datastore.
const SpecEntityName = "Spec"

// SpecsRegexp returns a regular expression that matches a collection of specs.
func SpecsRegexp() *regexp.Regexp {
	return regexp.MustCompile("^projects/" + nameRegex + "/products/" + nameRegex + "/versions/" + nameRegex + "/specs$")
}

// SpecRegexp returns a regular expression that matches a spec resource name.
func SpecRegexp() *regexp.Regexp {
	return regexp.MustCompile("^projects/" + nameRegex +
		"/products/" + nameRegex +
		"/versions/" + nameRegex +
		"/specs/" + nameRegex +
		revisionRegex + "$")
}

// Spec ...
type Spec struct {
	ProjectID   string    // Uniquely identifies a project.
	ProductID   string    // Uniquely identifies a product within a project.
	VersionID   string    // Uniquely identifies a version within a product.
	SpecID      string    // Uniquely identifies a spec within a version.
	RevisionID  string    // Uniquely identifies a revision of a spec.
	Description string    // A detailed description.
	CreateTime  time.Time // Creation time.
	UpdateTime  time.Time // Time of last change.
	Style       string    // Specification format.
	FileName    string    // Name of spec file.
	SizeInBytes int32     // Size of the spec file.
	Hash        string    // A hash of the spec file.
	SourceURI   string    // The original source URI of the spec file.
	Contents    []byte    `datastore:",noindex"` // The contents of the spec file.
}

// ParseParentVersion ...
func ParseParentVersion(parent string) ([]string, error) {
	r := regexp.MustCompile("^projects/" + nameRegex +
		"/products/" + nameRegex +
		"/versions/" + nameRegex +
		"$")
	m := r.FindAllStringSubmatch(parent, -1)
	if m == nil {
		return nil, fmt.Errorf("invalid version '%s'", parent)
	}
	return m[0], nil
}

// NewSpecFromParentAndSpecID returns an initialized spec for a specified parent and specID.
func NewSpecFromParentAndSpecID(parent string, specID string) (*Spec, error) {
	r := regexp.MustCompile("^projects/" + nameRegex +
		"/products/" + nameRegex +
		"/versions/" + nameRegex + "$")
	m := r.FindAllStringSubmatch(parent, -1)
	if m == nil {
		return nil, fmt.Errorf("invalid parent '%s'", parent)
	}
	if err := validateID(specID); err != nil {
		return nil, err
	}
	spec := &Spec{}
	spec.ProjectID = m[0][1]
	spec.ProductID = m[0][2]
	spec.VersionID = m[0][3]
	spec.SpecID = specID
	return spec, nil
}

// NewSpecFromResourceName parses resource names and returns an initialized spec.
func NewSpecFromResourceName(name string) (*Spec, error) {
	spec := &Spec{}
	m := SpecRegexp().FindAllStringSubmatch(name, -1)
	if m == nil {
		return nil, errors.New("invalid spec name")
	}
	spec.ProjectID = m[0][1]
	spec.ProductID = m[0][2]
	spec.VersionID = m[0][3]
	spec.SpecID = m[0][4]
	if strings.HasPrefix(m[0][5], "@") {
		spec.RevisionID = m[0][5][1:]
	}
	return spec, nil
}

// NewSpecFromMessage returns an initialized spec from a message.
func NewSpecFromMessage(message *rpc.Spec) (*Spec, error) {
	spec, err := NewSpecFromResourceName(message.GetName())
	if err != nil {
		return nil, err
	}
	spec.Description = message.GetDescription()
	spec.FileName = message.GetFilename()
	return spec, nil
}

// ResourceName generates the resource name of a spec.
func (spec *Spec) ResourceName() string {
	return fmt.Sprintf("projects/%s/products/%s/versions/%s/specs/%s",
		spec.ProjectID, spec.ProductID, spec.VersionID, spec.SpecID)
}

// ResourceNameWithRevision generates the resource name of a spec which includes the revision id.
func (spec *Spec) ResourceNameWithRevision() string {
	return fmt.Sprintf("projects/%s/products/%s/versions/%s/specs/%s@%s",
		spec.ProjectID, spec.ProductID, spec.VersionID, spec.SpecID, spec.RevisionID)
}

// ParentResourceName generates the resource name of a spec's parent.
func (spec *Spec) ParentResourceName() string {
	return fmt.Sprintf("projects/%s/products/%s/versions/%s", spec.ProjectID, spec.ProductID, spec.VersionID)
}

// Message returns a message representing a spec.
func (spec *Spec) Message(view rpc.SpecView, fullname bool) (message *rpc.Spec, err error) {
	message = &rpc.Spec{}
	if fullname {
		message.Name = spec.ResourceNameWithRevision()
	} else {
		message.Name = spec.ResourceName()
	}
	message.Filename = spec.FileName
	message.Description = spec.Description
	if view == rpc.SpecView_FULL {
		message.Contents = spec.Contents
	}
	message.Hash = spec.Hash
	message.SizeBytes = spec.SizeInBytes
	message.Style = spec.Style
	message.SourceUri = spec.SourceURI
	message.CreateTime, err = ptypes.TimestampProto(spec.CreateTime)
	message.UpdateTime, err = ptypes.TimestampProto(spec.UpdateTime)
	message.RevisionId = spec.RevisionID
	return message, err
}

// Update modifies a spec using the contents of a message.
func (spec *Spec) Update(message *rpc.Spec) error {
	now := time.Now()

	filename := message.GetFilename()
	if filename != "" {
		spec.FileName = filename
	}

	description := message.GetDescription()
	if description != "" {
		spec.Description = description
	}

	contents := message.GetContents()
	if contents != nil {
		spec.Contents = contents
		hash := hashForBytes(contents)
		if spec.Hash != hash {
			spec.Hash = hash
			spec.RevisionID = newRevisionID()
			spec.CreateTime = now
		}
		spec.SizeInBytes = int32(len(contents))
	}

	style := message.GetStyle()
	if style != "" {
		spec.Style = style
	}

	sourceURI := message.GetSourceUri()
	if sourceURI != "" {
		spec.SourceURI = sourceURI
	}

	spec.UpdateTime = now
	return nil
}

func newRevisionID() string {
	s := uuid.New().String()
	return s[len(s)-8:]
}

func hashForBytes(b []byte) string {
	h := sha1.New()
	h.Write(b)
	bs := h.Sum(nil)
	return fmt.Sprintf("%x", bs)
}
