// Copyright 2021 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"context"
	"fmt"
	"testing"

	"github.com/apigee/registry/rpc"
	"github.com/apigee/registry/server/registry/internal/test/seeder"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

var (
	// Example deployment api_spec_revision.
	deploymentApiSpecRevision = "/projects/p/apis/a/versions/v/specs/s@12345678"
)

func TestTagApiDeploymentRevision(t *testing.T) {
	ctx := context.Background()
	server := defaultTestServer(t)
	if err := seeder.SeedDeployments(ctx, server, &rpc.ApiDeployment{Name: "projects/my-project/locations/global/apis/my-api/deployments/d"}); err != nil {
		t.Fatalf("Setup/Seeding: Failed to seed registry: %s", err)
	}

	updateReq := &rpc.UpdateApiDeploymentRequest{
		ApiDeployment: &rpc.ApiDeployment{
			Name:            "projects/my-project/locations/global/apis/my-api/deployments/d",
			ApiSpecRevision: deploymentApiSpecRevision,
		},
	}

	revision, err := server.UpdateApiDeployment(ctx, updateReq)
	if err != nil {
		t.Fatalf("Setup: UpdateApiDeployment(%+v) returned error: %s", updateReq, err)
	}

	req := &rpc.TagApiDeploymentRevisionRequest{
		Name: fmt.Sprintf("%s@%s", revision.GetName(), revision.GetRevisionId()),
		Tag:  "my-tag",
	}

	got, err := server.TagApiDeploymentRevision(ctx, req)
	if err != nil {
		t.Fatalf("TagApiDeploymentRevision(%+v) returned error: %s", req, err)
	}

	opts := cmp.Options{
		protocmp.Transform(),
		protocmp.IgnoreFields(revision, "name", "revision_tags"),
	}

	t.Run("response", func(t *testing.T) {
		if !cmp.Equal(revision, got, opts) {
			t.Errorf("TagApiDeploymentRevision(%+v) returned unexpected diff (-want +got):\n%s", req, cmp.Diff(revision, got, opts))
		}

		if want := fmt.Sprintf("%s@my-tag", revision.GetName()); want != got.GetName() {
			t.Errorf("TagApiDeploymentRevision(%+v) returned unexpected name %q, want %q", req, got.GetName(), want)
		}
	})

	t.Run("GetApiDeployment", func(t *testing.T) {
		req := &rpc.GetApiDeploymentRequest{
			Name: got.GetName(),
		}

		got, err := server.GetApiDeployment(ctx, req)
		if err != nil {
			t.Fatalf("GetApiDeployment(%+v) returned error: %s", req, err)
		}

		if !cmp.Equal(revision, got, opts) {
			t.Errorf("GetApiDeployment(%+v) returned unexpected diff (-want +got):\n%s", req, cmp.Diff(revision, got, opts))
		}

		if got.GetName() != req.GetName() {
			t.Errorf("GetApiDeployment(%+v) returned unexpected name %q, want %q", req, got.GetName(), req.GetName())
		}
	})

	t.Run("add another tag to a tagged revision", func(t *testing.T) {
		req := &rpc.TagApiDeploymentRevisionRequest{
			Name: got.GetName(),
			Tag:  "my-second-tag",
		}

		got, err := server.TagApiDeploymentRevision(ctx, req)
		if err != nil {
			t.Fatalf("TagApiDeploymentRevision(%+v) returned error: %s", req, err)
		}

		opts := cmp.Options{
			protocmp.Transform(),
			protocmp.IgnoreFields(revision, "name", "revision_tags"),
		}

		if !cmp.Equal(revision, got, opts) {
			t.Errorf("TagApiDeploymentRevision(%+v) returned unexpected diff (-want +got):\n%s", req, cmp.Diff(revision, got, opts))
		}

		if want := fmt.Sprintf("%s@my-second-tag", revision.GetName()); want != got.GetName() {
			t.Errorf("TagApiDeploymentRevision(%+v) returned unexpected name %q, want %q", req, got.GetName(), want)
		}

		t.Run("GetApiDeployment", func(t *testing.T) {
			req := &rpc.GetApiDeploymentRequest{
				Name: got.GetName(),
			}

			got, err := server.GetApiDeployment(ctx, req)
			if err != nil {
				t.Fatalf("GetApiDeployment(%+v) returned error: %s", req, err)
			}

			if !cmp.Equal(revision, got, opts) {
				t.Errorf("GetApiDeployment(%+v) returned unexpected diff (-want +got):\n%s", req, cmp.Diff(revision, got, opts))
			}

			if got.GetName() != req.GetName() {
				t.Errorf("GetApiDeployment(%+v) returned unexpected name %q, want %q", req, got.GetName(), req.GetName())
			}
		})
	})

	t.Run("DeleteApiDeploymentRevision", func(t *testing.T) {
		req := &rpc.DeleteApiDeploymentRevisionRequest{
			Name: got.GetName(),
		}

		if _, err := server.DeleteApiDeploymentRevision(ctx, req); err != nil {
			t.Fatalf("DeleteApiDeploymentRevision(%+v) returned error: %s", req, err)
		}

		t.Run("GetApiDeployment", func(t *testing.T) {
			req := &rpc.GetApiDeploymentRequest{
				Name: req.GetName(),
			}

			if _, err := server.GetApiDeployment(ctx, req); status.Code(err) != codes.NotFound {
				t.Fatalf("GetApiDeployment(%+v) returned status code %q, want %q: %v", req, status.Code(err), codes.NotFound, err)
			}
		})
	})
}

func TestRollbackApiDeployment(t *testing.T) {
	ctx := context.Background()
	server := defaultTestServer(t)
	if err := seeder.SeedApis(ctx, server, &rpc.Api{Name: "projects/my-project/locations/global/apis/my-api"}); err != nil {
		t.Fatalf("Setup/Seeding: Failed to seed registry: %s", err)
	}

	createReq := &rpc.CreateApiDeploymentRequest{
		Parent:          "projects/my-project/locations/global/apis/my-api",
		ApiDeploymentId: "d",
		ApiDeployment:   &rpc.ApiDeployment{},
	}

	firstRevision, err := server.CreateApiDeployment(ctx, createReq)
	if err != nil {
		t.Fatalf("Setup: CreateApiDeployment(%+v) returned error: %s", createReq, err)
	}

	// Create a new revision so we can roll back from it.
	updateReq := &rpc.UpdateApiDeploymentRequest{
		ApiDeployment: &rpc.ApiDeployment{
			Name:            firstRevision.GetName(),
			ApiSpecRevision: deploymentApiSpecRevision,
		},
	}

	secondRevision, err := server.UpdateApiDeployment(ctx, updateReq)
	if err != nil {
		t.Fatalf("Setup: UpdateApiDeployment(%+v) returned error: %s", updateReq, err)
	}

	if secondRevision.GetRevisionId() == firstRevision.GetRevisionId() {
		t.Fatalf("Setup: UpdateApiDeployment(%+v) returned unexpected revision_id %q matching first revision, expected new revision ID", updateReq, secondRevision.GetRevisionId())
	}

	req := &rpc.RollbackApiDeploymentRequest{
		Name:       secondRevision.GetName(),
		RevisionId: firstRevision.GetRevisionId(),
	}

	rollback, err := server.RollbackApiDeployment(ctx, req)
	if err != nil {
		t.Fatalf("RollbackApiDeployment(%+v) returned error: %s", req, err)
	}

	want := &rpc.ApiDeployment{
		Name:               fmt.Sprintf("%s@%s", firstRevision.GetName(), rollback.GetRevisionId()),
		ApiSpecRevision:    firstRevision.GetApiSpecRevision(),
		CreateTime:         firstRevision.GetCreateTime(),
		RevisionCreateTime: firstRevision.GetRevisionCreateTime(),
		RevisionUpdateTime: firstRevision.GetRevisionUpdateTime(),
	}

	opts := cmp.Options{
		protocmp.Transform(),
		protocmp.IgnoreFields(new(rpc.ApiDeployment), "revision_id", "revision_create_time", "revision_update_time"),
	}

	if !cmp.Equal(want, rollback, opts) {
		t.Errorf("RollbackApiDeployment(%+v) returned unexpected diff (-want +got):\n%s", req, cmp.Diff(want, rollback, opts))
	}

	// Rollback should create a new revision, i.e. it should not reuse an existing revision ID.
	if rollback.GetRevisionId() == firstRevision.GetRevisionId() {
		t.Fatalf("RollbackApiDeployment(%+v) returned unexpected revision_id %q matching first revision, expected new revision ID", req, rollback.GetRevisionId())
	} else if rollback.GetRevisionId() == secondRevision.GetRevisionId() {
		t.Fatalf("RollbackApiDeployment(%+v) returned unexpected revision_id %q matching second revision, expected new revision ID", req, rollback.GetRevisionId())
	}
}

func TestDeleteApiDeploymentRevision(t *testing.T) {
	ctx := context.Background()
	server := defaultTestServer(t)
	if err := seeder.SeedDeployments(ctx, server, &rpc.ApiDeployment{Name: "projects/my-project/locations/global/apis/my-api/deployments/d"}); err != nil {
		t.Fatalf("Setup/Seeding: Failed to seed registry: %s", err)
	}

	t.Run("only remaining revision", func(t *testing.T) {
		t.Skip("not yet supported")

		req := &rpc.DeleteApiDeploymentRevisionRequest{
			Name: "projects/my-project/locations/global/apis/my-api/deployments/d",
		}

		if _, err := server.DeleteApiDeploymentRevision(ctx, req); status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("DeleteApiDeploymentRevision(%+v) returned unexpected status code %q, want %q: %v", req, status.Code(err), codes.FailedPrecondition, err)
		}
	})

	// Create a new revision so we can delete it.
	updateReq := &rpc.UpdateApiDeploymentRequest{
		ApiDeployment: &rpc.ApiDeployment{
			Name:            "projects/my-project/locations/global/apis/my-api/deployments/d",
			ApiSpecRevision: deploymentApiSpecRevision,
		},
	}

	secondRevision, err := server.UpdateApiDeployment(ctx, updateReq)
	if err != nil {
		t.Fatalf("Setup: UpdateApiDeployment(%+v) returned error: %s", updateReq, err)
	}

	t.Run("one of multiple existing revisions", func(t *testing.T) {
		req := &rpc.DeleteApiDeploymentRevisionRequest{
			Name: fmt.Sprintf("projects/my-project/locations/global/apis/my-api/deployments/d@%s", secondRevision.GetRevisionId()),
		}

		if _, err := server.DeleteApiDeploymentRevision(ctx, req); err != nil {
			t.Fatalf("DeleteApiDeploymentRevision(%+v) returned error: %s", req, err)
		}

		t.Run("GetApiDeployment", func(t *testing.T) {
			req := &rpc.GetApiDeploymentRequest{
				Name: req.GetName(),
			}

			if _, err := server.GetApiDeployment(ctx, req); status.Code(err) != codes.NotFound {
				t.Fatalf("GetApiDeployment(%+v) returned status code %q, want %q: %v", req, status.Code(err), codes.NotFound, err)
			}
		})
	})
}

func TestListApiDeploymentRevisions(t *testing.T) {
	ctx := context.Background()
	server := defaultTestServer(t)
	if err := seeder.SeedApis(ctx, server, &rpc.Api{Name: "projects/my-project/locations/global/apis/my-api"}); err != nil {
		t.Fatalf("Setup/Seeding: Failed to seed registry: %s", err)
	}

	createReq := &rpc.CreateApiDeploymentRequest{
		Parent:          "projects/my-project/locations/global/apis/my-api",
		ApiDeploymentId: "d",
		ApiDeployment:   &rpc.ApiDeployment{},
	}

	firstRevision, err := server.CreateApiDeployment(ctx, createReq)
	if err != nil {
		t.Fatalf("Setup: CreateApiDeployment(%+v) returned error: %s", createReq, err)
	}

	firstWant := &rpc.ApiDeployment{
		Name:               fmt.Sprintf("%s@%s", firstRevision.GetName(), firstRevision.GetRevisionId()),
		CreateTime:         firstRevision.GetCreateTime(),
		RevisionCreateTime: firstRevision.GetRevisionCreateTime(),
		RevisionUpdateTime: firstRevision.GetRevisionUpdateTime(),
		RevisionId:         firstRevision.GetRevisionId(),
	}

	updateReq := &rpc.UpdateApiDeploymentRequest{
		ApiDeployment: &rpc.ApiDeployment{
			Name:            firstRevision.GetName(),
			ApiSpecRevision: deploymentApiSpecRevision,
		},
	}

	secondRevision, err := server.UpdateApiDeployment(ctx, updateReq)
	if err != nil {
		t.Fatalf("Setup: UpdateApiDeployment(%+v) returned error: %s", updateReq, err)
	}

	secondWant := &rpc.ApiDeployment{
		Name:               fmt.Sprintf("%s@%s", secondRevision.GetName(), secondRevision.GetRevisionId()),
		ApiSpecRevision:    deploymentApiSpecRevision,
		CreateTime:         secondRevision.GetCreateTime(),
		RevisionCreateTime: secondRevision.GetRevisionCreateTime(),
		RevisionUpdateTime: secondRevision.GetRevisionUpdateTime(),
		RevisionId:         secondRevision.GetRevisionId(),
	}

	opts := cmp.Options{
		protocmp.Transform(),
	}

	var nextToken string
	t.Run("first page", func(t *testing.T) {
		req := &rpc.ListApiDeploymentRevisionsRequest{
			Name:     firstRevision.GetName(),
			PageSize: 1,
		}

		got, err := server.ListApiDeploymentRevisions(ctx, req)
		if err != nil {
			t.Fatalf("ListApiDeploymentRevisions(%+v) returned error: %s", req, err)
		}

		if count := len(got.GetApiDeployments()); count != 1 {
			t.Errorf("ListApiDeploymentRevisions(%+v) returned %d specs, expected exactly one", req, count)
		}

		// Check that the most recent revision is returned.
		want := []*rpc.ApiDeployment{secondWant}
		if !cmp.Equal(want, got.GetApiDeployments(), opts) {
			t.Errorf("List sequence returned unexpected diff (-want +got):\n%s", cmp.Diff(want, got.GetApiDeployments(), opts))
		}

		if got.GetNextPageToken() == "" {
			t.Errorf("ListApiDeploymentRevisions(%+v) returned empty next_page_token, expected another page", req)
		}

		nextToken = got.GetNextPageToken()
	})

	if t.Failed() {
		t.Fatal("Cannot test final page after failure on first page")
	}

	t.Run("final page", func(t *testing.T) {
		req := &rpc.ListApiDeploymentRevisionsRequest{
			Name:      firstRevision.GetName(),
			PageToken: nextToken,
		}

		got, err := server.ListApiDeploymentRevisions(ctx, req)
		if err != nil {
			t.Fatalf("ListApiDeploymentRevisions(%+v) returned error: %s", req, err)
		}

		if count := len(got.GetApiDeployments()); count != 1 {
			t.Errorf("ListApiDeploymentRevisions(%+v) returned %d specs, expected exactly one", req, count)
		}

		// Check that the original revision is returned.
		want := []*rpc.ApiDeployment{firstWant}
		if !cmp.Equal(want, got.GetApiDeployments(), opts) {
			t.Errorf("List sequence returned unexpected diff (-want +got):\n%s", cmp.Diff(want, got.GetApiDeployments(), opts))
		}

		if got.GetNextPageToken() != "" {
			t.Errorf("ListApiDeploymentRevisions(%+v) returned next_page_token, expected no next page", req)
		}
	})
}

func TestUpdateApiDeploymentRevisions(t *testing.T) {
	ctx := context.Background()
	server := defaultTestServer(t)
	if err := seeder.SeedApis(ctx, server, &rpc.Api{Name: "projects/my-project/locations/global/apis/my-api"}); err != nil {
		t.Fatalf("Setup/Seeding: Failed to seed registry: %s", err)
	}

	createReq := &rpc.CreateApiDeploymentRequest{
		Parent:          "projects/my-project/locations/global/apis/my-api",
		ApiDeploymentId: "my-spec",
		ApiDeployment: &rpc.ApiDeployment{
			Description: "Empty First Revision",
		},
	}

	created, err := server.CreateApiDeployment(ctx, createReq)
	if err != nil {
		t.Fatalf("Setup: CreateApiDeployment(%+v) returned error: %s", createReq, err)
	}

	opts := cmp.Options{
		protocmp.Transform(),
		protocmp.IgnoreFields(new(rpc.ApiDeployment), "revision_id", "create_time", "revision_create_time", "revision_update_time"),
	}

	t.Run("modify revision without content changes", func(t *testing.T) {
		req := &rpc.UpdateApiDeploymentRequest{
			ApiDeployment: &rpc.ApiDeployment{
				Name: created.GetName(),
			},
		}

		got, err := server.UpdateApiDeployment(ctx, req)
		if err != nil {
			t.Fatalf("UpdateApiDeployment(%+v) returned error: %s", req, err)
		}

		if got.GetRevisionId() != created.GetRevisionId() {
			t.Errorf("UpdateApiDeployment(%+v) returned unexpected revision_id %q, expected no change (%q)", req, got.GetRevisionId(), created.GetRevisionId())
		}

		if ct, ut := got.GetRevisionCreateTime().AsTime(), got.GetRevisionUpdateTime().AsTime(); !ct.Before(ut) {
			t.Errorf("UpdateApiDeployment(%+v) returned unexpected timestamps, expected revision_update_time %v > revision_create_time %v", req, ut, ct)
		}
	})

	t.Run("modify revision with api_spec_revision changes", func(t *testing.T) {
		req := &rpc.UpdateApiDeploymentRequest{
			ApiDeployment: &rpc.ApiDeployment{
				Name:            created.GetName(),
				ApiSpecRevision: deploymentApiSpecRevision,
			},
		}
		want := proto.Clone(created).(*rpc.ApiDeployment)
		want.ApiSpecRevision = req.GetApiDeployment().GetApiSpecRevision()

		got, err := server.UpdateApiDeployment(ctx, req)
		if err != nil {
			t.Fatalf("UpdateApiDeployment(%+v) returned error: %s", req, err)
		}

		if !cmp.Equal(want, got, opts) {
			t.Errorf("UpdateApiDeployment(%+v) returned unexpected diff (-want +got):\n%s", req, cmp.Diff(want, got, opts))
		}

		if got.GetRevisionId() == created.GetRevisionId() {
			t.Errorf("UpdateApiDeployment(%+v) returned unexpected revision_id %q, expected new revision", req, got.GetRevisionId())
		}

		if ct, ut := got.GetCreateTime().AsTime(), got.GetRevisionUpdateTime().AsTime(); !ct.Before(ut) {
			t.Errorf("UpdateApiDeployment(%+v) returned unexpected timestamps, expected revision_update_time %v > create_time %v", req, ut, ct)
		}
	})

	t.Run("modify specific revision", func(t *testing.T) {
		req := &rpc.UpdateApiDeploymentRequest{
			ApiDeployment: &rpc.ApiDeployment{
				Name: fmt.Sprintf("%s@%s", created.GetName(), created.GetRevisionId()),
			},
		}

		if _, err := server.UpdateApiDeployment(ctx, req); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("UpdateApiDeployment(%+v) returned unexpected status code %q, want %q: %v", req, status.Code(err), codes.InvalidArgument, err)
		}
	})
}
