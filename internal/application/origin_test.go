package application

import (
	"reflect"
	"testing"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

func Test_buildBodyPatch(t *testing.T) {
	type args struct {
		origin models.ApplicationOrigin
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "origin none",
			args: args{origin: models.ApplicationOrigin{
				Kind: models.OriginNone,
			}},
			want: `[{"op":"replace","path":"/spec/origin","value":{}}]`,
		},
		{
			name: "origin path",
			args: args{origin: models.ApplicationOrigin{
				Kind: models.OriginPath,
				Path: `C:\Documents\app`,
			}},
			want: `[{"op":"replace","path":"/spec/origin","value":{"path":"C:\\Documents\\app"}}]`,
		},
		{
			name: "origin container",
			args: args{origin: models.ApplicationOrigin{
				Kind:      models.OriginContainer,
				Container: "my-container",
			}},
			want: `[{"op":"replace","path":"/spec/origin","value":{"container":"my-container"}}]`,
		},
		{
			name: "origin git",
			args: args{origin: models.ApplicationOrigin{
				Kind: models.OriginGit,
				Git: models.GitRef{
					URL: "git@repo",
				},
			}},
			want: `[{"op":"replace","path":"/spec/origin","value":{"git":{"repository":"git@repo"}}}]`,
		},
		{
			name: "origin git with revision",
			args: args{origin: models.ApplicationOrigin{
				Kind: models.OriginGit,
				Git: models.GitRef{
					URL:      "git@repo",
					Revision: "revision_1",
				},
			}},
			want: `[{"op":"replace","path":"/spec/origin","value":{"git":{"repository":"git@repo","revision":"revision_1"}}}]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildBodyPatch(tt.args.origin)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildBodyPatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			result := string(got)
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("buildBodyPatch() = %v, want %v", result, tt.want)
			}
		})
	}
}
