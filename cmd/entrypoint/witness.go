package main

import (
	"context"
	"fmt"
	"os"

	"github.com/testifysec/go-witness"
	"github.com/testifysec/go-witness/archivista"
	"github.com/testifysec/go-witness/attestation"
	"github.com/testifysec/go-witness/attestation/commandrun"
	"github.com/testifysec/go-witness/attestation/material"
	"github.com/testifysec/go-witness/attestation/product"
	"github.com/testifysec/go-witness/cryptoutil"
	"github.com/testifysec/go-witness/dsse"
	"github.com/testifysec/go-witness/log"
	"github.com/testifysec/go-witness/signer/fulcio"
	"github.com/testifysec/go-witness/timestamp"
)

var FULCIO_URL = "https://v1.fulcio.sigstore.dev"
var FULCIO_OIDC_ISSUER = "https://oauth2.sigstore.dev/auth"
var FULCIO_OIDC_CLIENT_ID = "sigstore"

const TIMESTAMP_URL = "https://freetsa.org/tsr"
const ENABLE_TRACING = false
const ARCHIVISTA_URL = "https://archivista.testifysec.io"
const STEP_ENV = "TEKTON_RESOURCE_NAME"

func loadFulcioSigner(ctx context.Context) (cryptoutil.Signer, error) {

	signer, err := fulcio.Signer(ctx, FULCIO_URL, FULCIO_OIDC_ISSUER, FULCIO_OIDC_CLIENT_ID, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load fulcio signer: %w", err)
	}

	return signer, nil

}

func withWitness(ctx context.Context, args []string) error {
	signer, error := loadFulcioSigner(ctx)
	if error != nil {

		return fmt.Errorf("failed to load fulcio signer: %w", error)
	}

	timestampers := []dsse.Timestamper{}
	for _, url := range []string{TIMESTAMP_URL} {
		timestampers = append(timestampers, timestamp.NewTimestamper(timestamp.TimestampWithUrl(url)))
	}

	attestors := []attestation.Attestor{product.New(), material.New()}
	if len(args) > 0 {
		attestors = append(attestors, commandrun.New(commandrun.WithCommand(args), commandrun.WithTracing(ENABLE_TRACING)))
	}

	stepName := os.Getenv(STEP_ENV)
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	result, err := witness.Run(
		stepName,
		signer,
		witness.RunWithAttestors(attestors),
		witness.RunWithAttestationOpts(attestation.WithWorkingDir(workDir)),
		witness.RunWithTimestampers(timestampers...),
	)

	if err != nil {
		return err
	}

	//upload to archivista
	archivistaClient := archivista.New(ARCHIVISTA_URL)
	if gitoid, err := archivistaClient.Store(ctx, result.SignedEnvelope); err != nil {
		return fmt.Errorf("failed to store artifact in archivist: %w", err)
	} else {
		log.Infof("Stored in archivist as %v\n", gitoid)
	}

	return nil
}
