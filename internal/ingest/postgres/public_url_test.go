package postgres

import "testing"

func TestRebuildPublicImageURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		base string
		want string
	}{
		{
			name: "endpoint S3 con bucket",
			raw:  "https://19e79842547693ffe34fbd1d311d25dc.r2.cloudflarestorage.com/remesas-img/comprobantes/abc.jpg",
			base: "https://pub-49b9c87f6e6a418ba42de5ba36ddc73e.r2.dev",
			want: "https://pub-49b9c87f6e6a418ba42de5ba36ddc73e.r2.dev/comprobantes/abc.jpg",
		},
		{
			name: "base sin esquema",
			raw:  "https://account.r2.cloudflarestorage.com/remesas-img/comprobantes/abc.jpg",
			base: "pub-49b9c87f6e6a418ba42de5ba36ddc73e.r2.dev/",
			want: "https://pub-49b9c87f6e6a418ba42de5ba36ddc73e.r2.dev/comprobantes/abc.jpg",
		},
		{
			name: "URL pública existente no se toca",
			raw:  "https://pub-49b9c87f6e6a418ba42de5ba36ddc73e.r2.dev/comprobantes/abc.jpg",
			base: "https://pub-49b9c87f6e6a418ba42de5ba36ddc73e.r2.dev",
			want: "https://pub-49b9c87f6e6a418ba42de5ba36ddc73e.r2.dev/comprobantes/abc.jpg",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := rebuildPublicImageURL(tc.raw, tc.base, "remesas-img"); got != tc.want {
				t.Fatalf("url = %q, se esperaba %q", got, tc.want)
			}
		})
	}
}
