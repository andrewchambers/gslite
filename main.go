package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"

	"cloud.google.com/go/storage"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/iterator"
)

type Url struct {
	Bucket string
	Path   string
}

const urlRegexpStr = "^gs://([^/]+)/?(.*)$"

var urlRegexp = regexp.MustCompile(urlRegexpStr)

func parseUrl(url string) (Url, error) {
	match := urlRegexp.FindStringSubmatch(url)
	if match == nil {
		return Url{}, errors.New("Url did not match pattern: " + urlRegexpStr)
	}

	return Url{
		Bucket: match[1],
		Path:   match[2],
	}, nil
}

func Usage() int {
	fmt.Println(`
gslite - Small google storage client.

  gslite cat [gs://BUCKET/OBJECT ...]
    Print the concatenation of storage objects.

  gslite put gs://BUCKET/OBJECT
    Upload stdin to bucket.

  gslite stat [-compact] gs://BUCKET/OBJECT
    Print object information to stdout. Exit code is 2
    if the object did not exist, exit code is 1 on other errors.

  gslite exists gs://BUCKET/OBJECT
    Exit cleanly if the given object exists.

  gslite list [-stat] gs://BUCKET/OBJECT
    Print all object information under the given path.

  gslite rm [-r] gs://BUCKET/OBJECT
    Remove an object, do nothing sucessfully if it didn't exist.
    If -r is specified, will delete following the same
    rules that list follows.

  gslite mb [-storage-class=CLASS]
            [-location=LOC]
            [-public-access-prevention=inherited|enforced]
            [-google-cloud-project=PROJECT] NAME
    Create a bucket, project defaults to $GOOGLE_CLOUD_PROJECT.

  gslite rmb gs://BUCKET/
    Delete a bucket, do nothing successfully if it didn't exist.
  `)
	return 0
}

func Cat() int {

	flag.Parse()

	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}

	for _, arg := range flag.Args() {
		err := func() error {
			url, err := parseUrl(arg)
			if err != nil {
				return err
			}

			or, err := client.Bucket(url.Bucket).Object(url.Path).NewReader(ctx)
			if err != nil {
				return err
			}
			defer or.Close()

			_, err = io.Copy(os.Stdout, or)
			if err != nil {
				return err
			}

			return nil
		}()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return 1
		}
	}

	return 0
}

func Put() int {
	flag.Parse()

	if len(flag.Args()) != 1 {
		Usage()
		return 1
	}

	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}

	u, err := parseUrl(flag.Args()[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ow := client.Bucket(u.Bucket).Object(u.Path).NewWriter(ctx)

	_, err = io.Copy(ow, os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}

	err = ow.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}

	return 0
}

func Stat() int {

	compact := flag.Bool("compact", false, "Print json in a compact format.")

	flag.Parse()

	if len(flag.Args()) != 1 {
		Usage()
		return 1
	}

	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}

	u, err := parseUrl(flag.Args()[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}

	st, err := client.Bucket(u.Bucket).Object(u.Path).Attrs(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		if err == storage.ErrObjectNotExist || err == storage.ErrBucketNotExist {
			return 2
		}
		return 1
	}

	var buf []byte

	if *compact {
		buf, err = json.Marshal(st)
	} else {
		buf, err = json.MarshalIndent(st, "", "  ")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}
	_, err = os.Stdout.Write(buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}

	return 0
}

func List() int {

	jsonl := flag.Bool("jsonl", false, "Print in jsonl format.")

	flag.Parse()

	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}

	for _, arg := range flag.Args() {

		u, err := parseUrl(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return 1
		}

		it := client.Bucket(u.Bucket).Objects(ctx, &storage.Query{
			Prefix: u.Path,
		})
		for {
			oattr, err := it.Next()
			if err != nil {
				if err == iterator.Done {
					break
				}
				fmt.Fprintf(os.Stderr, "%s\n", err)
				return 1
			}
			if *jsonl {
				buf, err := json.Marshal(oattr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s\n", err)
					return 1
				}
				_, err = fmt.Fprintf(os.Stdout, "%s\n", string(buf))
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s\n", err)
					return 1
				}
			} else {
				_, err = fmt.Fprintf(os.Stdout, "gs://%s/%s\n", oattr.Bucket, oattr.Name)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s\n", err)
					return 1
				}
			}
		}
	}

	return 0
}

func Rm() int {

	r := flag.Bool("r", false, "Delete all objects with this prefix.")
	j := flag.Int("j", 64, "Maximum number of concurrent delete calls to perform.")

	flag.Parse()

	if *j <= 0 {
		*j = 1
	}

	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}

	for _, arg := range flag.Args() {
		u, err := parseUrl(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return 1
		}

		bucket := client.Bucket(u.Bucket)

		if *r {
			sem := semaphore.NewWeighted(int64(*j))
			errg, ctx := errgroup.WithContext(ctx)

			it := bucket.Objects(ctx, &storage.Query{
				Prefix: u.Path,
			})

			for {
				oattr, err := it.Next()
				if err != nil {
					if err == iterator.Done {
						break
					}
					if workerErr := errg.Wait(); workerErr != nil {
						err = workerErr
					}
					fmt.Fprintf(os.Stderr, "%s\n", err)
					return 1
				}
				o := bucket.Object(oattr.Name)
				err = sem.Acquire(ctx, 1)
				if err != nil {
					// The only reason this would happen is a worker failed.
					err = errg.Wait()
					fmt.Fprintf(os.Stderr, "%s\n", err)
					return 1
				}
				errg.Go(func() error {
					defer sem.Release(1)
					err := o.Delete(ctx)
					if err != nil {
						if err != storage.ErrObjectNotExist {
							return err
						}
					}
					return nil
				})
			}

			err = errg.Wait()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				return 1
			}

		} else {
			o := client.Bucket(u.Bucket).Object(u.Path)
			err = o.Delete(ctx)
			if err != nil {
				if err != storage.ErrObjectNotExist {
					fmt.Fprintf(os.Stderr, "%s\n", err)
					return 1
				}
			}
		}
		return 0
	}

	return 0
}

func Mb() int {

	project := flag.String("google-cloud-project", "", "Project to make bucket under, defaults to $GOOGLE_CLOUD_PROJECT.")
	location := flag.String("location", "US", "Bucket location.")
	storageClass := flag.String("storage-class", "STANDARD", "Bucket default storage class.")
	publicAccessPrevention := flag.String("public-access-prevention", "inherited", "Public access prevention, one of 'inherited' or 'enforced'.")

	flag.Parse()

	if *project == "" {
		*project = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}

	var parsedPublicAccessPrevention storage.PublicAccessPrevention
	switch *publicAccessPrevention {
	case "inherited":
		parsedPublicAccessPrevention = storage.PublicAccessPreventionInherited
	case "enforced":
		parsedPublicAccessPrevention = storage.PublicAccessPreventionEnforced
	default:
		fmt.Fprintf(os.Stderr, "expected one of inherited or enforced, got %s\n", *publicAccessPrevention)
		return 1
	}

	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}

	for _, arg := range flag.Args() {
		u, err := parseUrl(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return 1
		}
		err = client.Bucket(u.Bucket).Create(ctx, *project, &storage.BucketAttrs{
			Location:               *location,
			StorageClass:           *storageClass,
			PublicAccessPrevention: parsedPublicAccessPrevention,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return 1
		}
	}

	return 0
}

func Rmb() int {

	flag.Parse()

	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}

	for _, arg := range flag.Args() {
		u, err := parseUrl(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return 1
		}
		err = client.Bucket(u.Bucket).Delete(ctx)
		if err != nil {
			if err != storage.ErrBucketNotExist {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				return 1
			}
		}
	}

	return 0
}

func main() {

	flag.Usage = func() { Usage() }

	if len(os.Args) <= 1 {
		Usage()
		os.Exit(1)
	}

	cmd := Usage

	switch os.Args[1] {
	case "cat":
		cmd = Cat
	case "put":
		cmd = Put
	case "stat":
		cmd = Stat
	case "list":
		cmd = List
	case "rm":
		cmd = Rm
	case "mb":
		cmd = Mb
	case "rmb":
		cmd = Rmb
	case "help":
		cmd = Usage
	default:
		_ = Usage()
		os.Exit(1)
	}

	copy(os.Args[0:], os.Args[1:])
	os.Args = os.Args[:len(os.Args)-1]
	os.Exit(cmd())
}
