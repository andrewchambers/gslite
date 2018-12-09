# gslite

A lightweight alternative to google cloud sdk gsutil.

# Rationale

gsutil from google is a 200 mb dependency, and doesn't have reliable ways to
check if an object doesn't exist.

gslite solves both of those problems.

# Usage

```
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
    Remove an object, do nothing if it didn't exist.
    If -r is specified, will delete following the same
    rules that list follows.

  gslite mb -project PROJECT NAME
    Create a bucket.

  gslite rmb gs://BUCKET/
    Delete a bucket.
```