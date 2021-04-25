# Extensions and integrations

Each sub-directory is a go module, with specific dependencies. 
The top level repo should have no external dependencies.

The /cmd/ugate package imports each module, using a separate ugate_NAME file and
NAME or NO_NAME tag for conditional compilation. Also costinm/dmesh repo imports
the modules used on Android.
