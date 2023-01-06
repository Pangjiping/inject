package singleton_pointer

import (
	"encoding/json"
	"github.com/Pangjiping/inject"
	"testing"
)

func TestForSingletonPointerDependencyInject(t *testing.T) {
	//logger := test.NewDebugLogger()

	// init root
	config := loadConfig()
	db := connectDB()

	// create leaf [don't init]
	server := Server{}

	//graph := inject.Graph{Logger: logger}
	graph := inject.Graph{}

	if err := graph.Provide(
		&inject.Object{
			Value: &server,
		},
		&inject.Object{
			Value: config,
		},
		&inject.Object{
			Value: db,
		},
	); err != nil {
		t.Fatal(err)
	}

	if err := graph.Populate(); err != nil {
		t.Fatal(err)
	}

	bytes, err := json.Marshal(server)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(bytes))

}
