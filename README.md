# VPN Simulation

Simulation of a VPN application using Golang.

## How to use

Before running the application there are certain settngs that need to be configured.
__NOTE: Instruction for Windows and Linux not available at the time.__

### MacOS

1. Run the server code in its own terminal.
2. Run the client code in its own terminal.
3. After running the client code, a new network tunnel is created with the name `utun13`. This tunnel needs to be added to the routes on your machine network table. To do so, you need to do the following step:
    a. Run command `sudo ifconfig utun13 inet 10.0.0.1 10.0.0.2 up`
        - `10.0.0.1` is your local machine
        - `10.0.0.2` is the IP of this application, and set the flag up so it's up and running
    b. Run command `sudo route add 0.0.0.0/0 -interface utun13`
        - we're saying whatever data/packets received from or sent to your machine would directly go into this Go client app
        - basically, you have rerouted the traffic of your machine to go through this app

