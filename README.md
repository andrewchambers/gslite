# gslite

A lightweight alternative to google cloud sdk gsutil that provides 
a simple and consistent interface.

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
```