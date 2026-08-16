package main

import (
	"bytes"
	"context"
	"fmt"

	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/xslt3"
)

func ProcessXSLT(xmlInput, xsltInput []byte, diagram *string, source *string) ([]byte, error) {
	ctx := context.Background()
	parser := helium.NewParser()

	// 1. Parse the XSLT stylesheet into a *helium.Document
	stylesheetDoc, err := parser.Parse(ctx, xsltInput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XSLT input: %w", err)
	}

	// 2. Compile the parsed stylesheet document
	stylesheet, err := xslt3.CompileStylesheet(ctx, stylesheetDoc)
	if err != nil {
		return nil, fmt.Errorf("failed to compile XSLT stylesheet: %w", err)
	}

	// 3. Parse the target XML document
	sourceDoc, err := parser.Parse(ctx, xmlInput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse source XML input: %w", err)
	}

	// 4. Configure transformation parameters
	inv := stylesheet.Transform(sourceDoc)
	if (diagram != nil && *diagram != "") || (source != nil && *source != "") {
		params := xslt3.NewParameters()
		if diagram != nil {
			params.SetString("diagram", *diagram)
		}
		params.SetString("file", *source)

		// Example of passing multiple parameters:
		// params.SetString("anotherParam", "someValue")
		// params.SetString("debugMode", "true")

		inv = inv.GlobalParameters(params)
	}

	// 5. Transform the document and write serialized output to a buffer
	var buf bytes.Buffer
	if err := inv.WriteTo(ctx, &buf); err != nil {
		return nil, fmt.Errorf("failed to execute XSLT transformation: %w", err)
	}

	return buf.Bytes(), nil
}
