// Package cloudports builds the data ports of the gcp and emulator targets
// from the cloud adapters, so every service and job wires them the same way.
// It imports no local-only adapter.
//
// Settings come from the environment: GCP_PROJECT, GCS_BUCKET, and optionally
// FIRESTORE_DATABASE. The emulator target also needs STORAGE_EMULATOR_HOST and
// FIRESTORE_EMULATOR_HOST (the client libraries read them), defaults the
// project and bucket, and creates the bucket. The gcp target refuses to start
// when either emulator host is set, so a stray variable can never point a
// deployed service at an emulator, nor an emulator run at a real project.
package cloudports

import (
	"cmp"
	"context"
	"errors"
	"fmt"

	"github.com/easterthebunny/pubcommittee/internal/adapters/firestore"
	"github.com/easterthebunny/pubcommittee/internal/adapters/gcs"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Target is a build target that uses the cloud adapters.
type Target string

// Targets.
const (
	GCP      Target = "gcp"
	Emulator Target = "emulator"
)

const (
	emulatorProject = "pc-emulator"
	emulatorBucket  = "pc-emulator-bucket"
)

// Ports holds the two cloud adapters.
type Ports struct {
	Objects *gcs.Store
	Docs    *firestore.Store
}

// settings returns the project and bucket of target, or ErrInvalid.
func settings(target Target, getenv func(string) string) (project, bucket string, err error) {
	project, bucket = getenv("GCP_PROJECT"), getenv("GCS_BUCKET")
	emulated := getenv("STORAGE_EMULATOR_HOST") != "" || getenv("FIRESTORE_EMULATOR_HOST") != ""
	switch target {
	case GCP:
		if emulated {
			return "", "", fmt.Errorf("gcp target: an emulator host is set: %w", data.ErrInvalid)
		}
		if project == "" || bucket == "" {
			return "", "", fmt.Errorf("gcp target: GCP_PROJECT and GCS_BUCKET are required: %w", data.ErrInvalid)
		}
	case Emulator:
		if getenv("STORAGE_EMULATOR_HOST") == "" || getenv("FIRESTORE_EMULATOR_HOST") == "" {
			return "", "", fmt.Errorf(
				"emulator target: STORAGE_EMULATOR_HOST and FIRESTORE_EMULATOR_HOST are required: %w", data.ErrInvalid)
		}
		project, bucket = cmp.Or(project, emulatorProject), cmp.Or(bucket, emulatorBucket)
	default:
		return "", "", fmt.Errorf("unknown target %q: %w", target, data.ErrInvalid)
	}
	return project, bucket, nil
}

// Open validates the environment for target and builds the adapters. getenv is
// os.Getenv in services.
func Open(ctx context.Context, target Target, getenv func(string) string) (*Ports, error) {
	project, bucket, err := settings(target, getenv)
	if err != nil {
		return nil, err
	}
	objects, err := gcs.New(ctx, gcs.Config{Bucket: bucket})
	if err != nil {
		return nil, fmt.Errorf("objects: %w", err)
	}
	docs, err := firestore.New(ctx, firestore.Config{Project: project, Database: getenv("FIRESTORE_DATABASE")})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("docs: %w", err), objects.Close())
	}
	if target == Emulator {
		if err := objects.EnsureBucket(ctx, project); err != nil {
			return nil, errors.Join(fmt.Errorf("emulator bucket: %w", err), objects.Close(), docs.Close())
		}
	}
	return &Ports{Objects: objects, Docs: docs}, nil
}

// Close releases both adapters.
func (p *Ports) Close() error { return errors.Join(p.Objects.Close(), p.Docs.Close()) }

// Public returns the ports read-api holds.
func (p *Ports) Public() data.PublicReadPorts {
	return data.PublicReadPorts{
		BillText: p.Objects, PublicDocs: p.Docs, Comparisons: p.Objects, Citations: p.Objects, Law: p.Objects,
		Feed: p.Objects, Exports: p.Objects, Generated: data.NewGeneratedStore(p.Objects),
	}
}

// Platform returns the ports jobs and admin-api hold.
func (p *Ports) Platform() data.PlatformPorts {
	var ports data.PlatformPorts
	ports.PublicReadPorts = p.Public()
	store := data.NewGeneratedStore(p.Objects)
	ports.PlatformDocs, ports.Moderation, ports.Publisher = p.Docs, p.Docs, p.Objects
	ports.ComparisonStore, ports.CitationStore, ports.LawStore = p.Objects, p.Objects, p.Objects
	ports.FeedStore, ports.ExportStore, ports.GeneratedStore = p.Objects, p.Objects, store
	return ports
}
