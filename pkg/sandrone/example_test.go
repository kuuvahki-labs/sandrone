package sandrone_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

func ExampleEngine_Convert() {
	engine := sandrone.New()
	result, err := engine.Convert(context.Background(), sandrone.ConvertRequest{
		FromFormat: "uri-list",
		ToFormat:   "mihomo-proxies",
		Content:    []byte("ss://aes-128-gcm:secret@example.com:8388#node-a"),
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(result.ContentType)
	fmt.Println(strings.Contains(string(result.Body), "node-a"))

	// Output:
	// application/yaml
	// true
}
