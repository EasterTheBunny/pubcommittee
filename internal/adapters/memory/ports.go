package memory

import "github.com/easterthebunny/pubcommittee/internal/data"

// Compile-time port checks.
var (
	_ data.BillText     = (*Store)(nil)
	_ data.Publisher    = (*Store)(nil)
	_ data.PublicDocs   = (*Store)(nil)
	_ data.PlatformDocs = (*Store)(nil)
	_ data.Moderation   = (*Store)(nil)
	_ data.BillSource   = (*Store)(nil)
	_ data.LawSource    = (*Store)(nil)
	_ data.Generator    = (*Store)(nil)
)

// PlatformPorts returns the platform group backed by s.
func (s *Store) PlatformPorts() data.PlatformPorts {
	return data.PlatformPorts{
		PublicReadPorts: s.PublicReadPorts(),
		PlatformDocs:    s,
		Moderation:      s,
		Publisher:       s,
	}
}

// PublicReadPorts returns the public read group backed by s.
func (s *Store) PublicReadPorts() data.PublicReadPorts {
	return data.PublicReadPorts{BillText: s, PublicDocs: s}
}
