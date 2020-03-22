package config

import "testing"

func TestValidate(t *testing.T) {
	type args struct {
		config *Main
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "entrypoint is empty",
			args: args{
				config:&Main{
					Passthrough: false,
					Users: map[string]*User{
						"test": &User{
							Entrypoint: "",
							Sitemaps:   Sitemap{},
							Paths:      nil,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "default sitemap is empty",
			args: args{
				config:&Main{
					Passthrough: false,
					Users: map[string]*User{
						"test": &User{
							Entrypoint: "test",
							Sitemaps:   Sitemap{
								Default: "",
								Allowed: nil,
							},
							Paths:      nil,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "allowed sitemaps is empty",
			args: args{
				config:&Main{
					Passthrough: false,
					Users: map[string]*User{
						"test": &User{
							Entrypoint: "test",
							Sitemaps:   Sitemap{
								Default: "test",
								Allowed: nil,
							},
							Paths:      nil,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid config",
			args: args{
				config:&Main{
					Passthrough: false,
					Users: map[string]*User{
						"test": &User{
							Entrypoint: "test",
							Sitemaps:   Sitemap{
								Default: "test",
								Allowed: []string{"test"},
							},
							Paths:      nil,
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
