# Spin Cycle Docs

This directory contains the source docs for https://square.github.io/spincycle/.

## Development

Run `bundle exec jekyll serve --incremental` to serve docs locally.

## Search

To update the search index, run the following. You should do this after you modify any of the content in the docs.
```
bundle exec just-the-docs rake search:init
cp _site/assets/js/search-data.json assets/js/
chmod +x assets/js/search-data.json
```
