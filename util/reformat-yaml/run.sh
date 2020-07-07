#!/bin/bash

for filename in $(find ../.. -name '*.yaml'); do
	# Don't replace the reformatter test file!
	if [ "$filename" = "../../util/reformat-yaml/test/test.yaml" ]; then
		continue
	fi
	./reformat "$filename" ./temp.yaml
	cp ./temp.yaml "$filename"
done
rm ./temp.yaml
