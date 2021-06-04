#! /usr/bin/env bash

set -eux
set -o pipefail

go build -race

./gslite rm -r "gs://$TEST_BUCKET_NAME/" || true
./gslite rmb "gs://$TEST_BUCKET_NAME/" || true

./gslite mb -google-cloud-project "$TEST_PROJECT" "gs://$TEST_BUCKET_NAME/"

echo -n "foo" | ./gslite put "gs://$TEST_BUCKET_NAME/a"
echo -n "bar" | ./gslite put "gs://$TEST_BUCKET_NAME/b"

test "$(./gslite stat "gs://$TEST_BUCKET_NAME/a" | jq -r .Name)" = "a"
test "$(./gslite cat "gs://$TEST_BUCKET_NAME/"{a,b})" = "foobar"

listing=$(echo -e "gs://$TEST_BUCKET_NAME/a\ngs://$TEST_BUCKET_NAME/b\n")
test "$(./gslite list "gs://$TEST_BUCKET_NAME")" = "$listing"


set +e
./gslite stat BADURL://foobar/
rc1="$?"
./gslite stat "gs://$TEST_BUCKET_NAME/c"
rc2="$?"
set -e
test "$rc1" = 1
test "$rc2" = 2


./gslite rm "gs://$TEST_BUCKET_NAME/a"
# rm twice is still okay.
./gslite rm "gs://$TEST_BUCKET_NAME/a"

test "$(./gslite list "gs://$TEST_BUCKET_NAME")" = "gs://$TEST_BUCKET_NAME/b"
test "$(./gslite list -jsonl "gs://$TEST_BUCKET_NAME" | jq -r .Name)" = "b"

for i in $(seq 32)
do
  echo -n "data" | ./gslite put "gs://$TEST_BUCKET_NAME/o$i" &
done
wait 

./gslite rm -r -j 16 "gs://$TEST_BUCKET_NAME/"
./gslite rmb "gs://$TEST_BUCKET_NAME/"
