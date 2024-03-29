package gointerfacer

import (
	"reflect"
	"testing"
)

type errBool bool

func (b errBool) String() string {
	if b {
		return "an error"
	} else {
		return "no error"
	}
}

func TestFindInterface(t *testing.T) {
	cases := []struct {
		iface   string
		path    string
		pkg     string
		id      string
		wantErr bool
	}{
		{iface: "net.Conn", path: "net", pkg: "net", id: "Conn"},
		{iface: "http.ResponseWriter", path: "net/http", pkg: "http", id: "ResponseWriter"},
		{iface: "net.Tennis", wantErr: true},
		{iface: "a + b", wantErr: true},
	}

	for _, tt := range cases {
		path, pkg, id, err := FindInterface(tt.iface)
		gotErr := err != nil
		if tt.wantErr != gotErr {
			t.Errorf("FindInterface(%q).err=%v want %s", tt.iface, err, errBool(tt.wantErr))
			continue
		}
		if tt.path != path {
			t.Errorf("FindInterface(%q).path=%q want %q", tt.iface, path, tt.path)
		}
		if tt.pkg != pkg {
			t.Errorf("FindInterface(%q).pkg=%q want %q", tt.iface, pkg, tt.pkg)
		}
		if tt.id != id {
			t.Errorf("FindInterface(%q).id=%q want %q", tt.iface, id, tt.id)
		}
	}
}

func TestTypeSpec(t *testing.T) {
	// For now, just test whether we can find the interface.
	cases := []struct {
		path    string
		id      string
		wantErr bool
	}{
		{path: "net", id: "Conn"},
		{path: "net", id: "Con", wantErr: true},
	}

	for _, tt := range cases {
		p, spec, err := TypeSpec(tt.path, tt.id)
		gotErr := err != nil
		if tt.wantErr != gotErr {
			t.Errorf("TypeSpec(%q, %q).err=%v want %s", tt.path, tt.id, err, errBool(tt.wantErr))
			continue
		}
		if err == nil {
			if reflect.DeepEqual(p, Pkg{}) {
				t.Errorf("TypeSpec(%q, %q).pkg=Pkg{} want non-nil", tt.path, tt.id)
			}
			if spec == nil {
				t.Errorf("TypeSpec(%q, %q).spec=nil want non-nil", tt.path, tt.id)
			}
		}
	}
}

func TestFuncs(t *testing.T) {
	cases := []struct {
		iface   string
		want    []Func
		wantErr bool
	}{
		{
			iface: "io.ReadWriter",
			want: []Func{
				{
					Name:   "Read",
					Params: []Param{{Name: "p", Type: "[]byte"}},
					Res: []Param{
						{Name: "n", Type: "int"},
						{Name: "err", Type: "error"},
					},
				},
				{
					Name:   "Write",
					Params: []Param{{Name: "p", Type: "[]byte"}},
					Res: []Param{
						{Name: "n", Type: "int"},
						{Name: "err", Type: "error"},
					},
				},
			},
		},
		{
			iface: "http.ResponseWriter",
			want: []Func{
				{
					Name: "Header",
					Res:  []Param{{Type: "http.Header"}},
				},
				{
					Name:   "Write",
					Params: []Param{{Type: "[]byte"}},
					Res:    []Param{{Type: "int"}, {Type: "error"}},
				},
				{
					Name:   "WriteHeader",
					Params: []Param{{Name: "statusCode", Type: "int"}},
				},
			},
		},
		{
			iface: "http.Handler",
			want: []Func{
				{
					Name:   "ServeHTTP",
					Params: []Param{{Type: "http.ResponseWriter"}, {Type: "*http.Request"}},
				},
			},
		},
		{
			iface: "ast.Node",
			want: []Func{
				{
					Name: "Pos",
					Res:  []Param{{Type: "token.Pos"}},
				},
				{
					Name: "End",
					Res:  []Param{{Type: "token.Pos"}},
				},
			},
		},
		{
			iface: "cipher.AEAD",
			want: []Func{
				{
					Name: "NonceSize",
					Res:  []Param{{Type: "int"}},
				},
				{
					Name: "Overhead",
					Res:  []Param{{Type: "int"}},
				},
				{
					Name: "Seal",
					Params: []Param{
						{Name: "dst", Type: "[]byte"},
						{Name: "nonce", Type: "[]byte"},
						{Name: "plaintext", Type: "[]byte"},
						{Name: "additionalData", Type: "[]byte"},
					},
					Res: []Param{{Type: "[]byte"}},
				},
				{
					Name: "Open",
					Params: []Param{
						{Name: "dst", Type: "[]byte"},
						{Name: "nonce", Type: "[]byte"},
						{Name: "ciphertext", Type: "[]byte"},
						{Name: "additionalData", Type: "[]byte"},
					},
					Res: []Param{{Type: "[]byte"}, {Type: "error"}},
				},
			},
		},
		{iface: "net.Tennis", wantErr: true},
	}

	for _, tt := range cases {
		fns, err := Functions(tt.iface)
		gotErr := err != nil
		if tt.wantErr != gotErr {
			t.Errorf("Functions(%q).err=%v want %s", tt.iface, err, errBool(tt.wantErr))
			continue
		}
		if !reflect.DeepEqual(fns, tt.want) {
			t.Errorf("Functions(%q).fns=\n%v\nwant\n%v\n", tt.iface, fns, tt.want)
		}
	}
}
