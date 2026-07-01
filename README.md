# Semantic STEP Technology

This project contains the SST Core packages.

The only packages that are relevant for normal SST application developers are:
 - [sst](https:sst/sst): this contains the SST core API that every application has to use
 - [examples](https:sst/examples): here you can find small reference examples such as HelloWordRead and HelloWordWrite
 - [vocabularies](https:sst/vocabularies): this contains the pre-compiled higher level ontologies into early binding GO variables that simplify typical SST application programming. For typical SST applications you wont to any change here. Only when you need to support your own high level ontologies you might add or modify the sub-packaes.
 - [defaultderive](https:sst/defaultderive): this is only for experiensed power users of SST when you need to configures your own specific index mapping of derived NamedGraph data for Bleve.
 - [cmd](https:sst/cmd): this contains various command line tools, including a Command Line Interface (CLI) similar to the one of GIT.

All other packages are only used internally and must not be used directly by the user.


