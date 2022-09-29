package xtype

import "testing"

func Test_getTagFirstValue(t *testing.T) {
	type args struct {
		v string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			args: args{v: "id,omitempty"},
			want: "id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getTagFirstValue(tt.args.v); got != tt.want {
				t.Errorf("getTagFirstValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
