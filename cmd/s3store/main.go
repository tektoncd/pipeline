package main

import (
	"bufio"
	"context"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/kelseyhightower/envconfig"
	flag "github.com/spf13/pflag"
	"github.com/tektoncd/pipeline/pkg/logging"
	s3blob "gocloud.dev/blob/s3blob"
)

var (
	Get         bool
	Put         bool
	Bucket      string
	Artifact    string
	Destination string
	Source      string
	Region      string
)

type AWSStore struct {
	Timeout time.Duration `default:300`
}

func init() {
	flag.BoolVarP(&Get, "get", "g", false, "get object from bucket")
	flag.BoolVarP(&Put, "put", "p", false, "put object into bucket")
	flag.StringVarP(&Bucket, "bucket", "b", "", "bucket to connect to") // s3://my-bucket?region=us-west-1
	flag.StringVarP(&Artifact, "artifact", "a", "", "artifact to put or get")
	flag.StringVarP(&Destination, "destination", "d", "", "path to put the artifact")
	flag.StringVarP(&Source, "source", "s", "", "path to get the artifact to put")
	flag.StringVarP(&Region, "region", "r", "us-east-1", "region of the bucket. defaults to us-east-1")
}

func main() {
	flag.Parse()
	logger, _ := logging.NewLogger("", "s3store")
	defer logger.Sync()

	var s AWSStore
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatalf("error processing environment config error=%q", err)
	}
	if Bucket == "" {
		log.Fatal("--bucket must be set")
	}
	if Artifact == "" {
		log.Fatal("--artifact name must be set")
	}

	log.Printf("app=s3store bucket=%s artifact=%s destination=%s", Bucket, Artifact, Destination)
	switch {
	case Put:
		if Source == "" {
			log.Fatal("--source must be provided for Put operation")
		}

		if err := s.putArtifact(); err != nil {
			log.Fatalf("failed to put artifact: %s error=%q", Artifact, err)
		}
	case Get:
		if Destination == "" {
			log.Fatal("--destination must be provided for Get operation")
		}

		if err := s.getArtifact(); err != nil {
			log.Fatalf("failed to get artifact: %s error=%q", Artifact, err)
		}
	default:
		log.Fatal("one of --get or --put must be set")
	}

}

func (s *AWSStore) putArtifact() error {
	ctx := context.Background()

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(Region),
	})
	if err != nil {
		return err
	}

	bucket, err := s3blob.OpenBucket(ctx, sess, Bucket, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer bucket.Close()

	localSource, err := ioutil.ReadFile(Source)
	if err != nil {
		return err
	}
	return bucket.WriteAll(ctx, Artifact, localSource, nil)
}

func (s *AWSStore) getArtifact() error {
	log.Println("getting artifact")

	ctx := context.Background()

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(Region),
	})
	if err != nil {
		return err
	}

	bucket, err := s3blob.OpenBucket(ctx, sess, Bucket, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer bucket.Close()
	if err := os.MkdirAll(filepath.Dir(Destination), 0644); err != nil {
		log.Fatal(err)
	}
	tmpfile, err := ioutil.TempFile(filepath.Dir(Destination), "Destination")
	if err != nil {
		log.Fatal(err)
	}

	w := bufio.NewWriter(tmpfile)

	r, err := bucket.NewReader(ctx, Artifact, nil)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	w.Flush()
	if err := os.Rename(tmpfile.Name(), Destination); err != nil {
		return err
	}

	return nil
}
