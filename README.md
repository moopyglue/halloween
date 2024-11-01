# halloween

Either just run the process and it will ldefault to listen on port 7777 or set env variable LIST_PORT to choose your own

Original version always ran on 7777 but because it was run inside a container the container was modifed to map the port
to make it uniqie externally.

exmple runn from WSL or UNIX command line

$ LISTEN_PORT=8080 ./halloween

## compiling

clone this repo inyour GO directory under source, cd to the directoyr and run "go build ." and the executable will be generated

## user guide

in 3 seperate windows navigate to the port on a browser - you will be connected a menu with 3 options
- left eye
- right eye
- controller

The controller asumes touch input which on a non-touch device can be emulated if you wish using the developer tools.

The controller is intended to be run on a mobile phone and the controlelr page provides multitouchso you can open and close the eye at the same time as using it to look around

Here is a link to a youtube video of wwhat it looks like
https://youtu.be/pxn623gdWtI
