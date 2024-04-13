# VPN Simulation

Simulation of a VPN application using Golang.

## How to use

__FIRST:__ 
1. Run `netstat -nr | grep default`. You should see something like this (your broadcast en0 will be a address and NOT 000.000.0.000): 
```bash
default            000.000.0.000      UGScg                 en0 
```
Remember this address so you can revert the setting since this app might tamper with your network configurations.


Before running the application there are certain settngs that need to be configured.
__NOTE: Instruction for Windows and Linux not available at the time.__

### MacOS

1. Run the server code in its own terminal.
2. Run the client code in its own terminal.
3. Run `sudo route delete default` to delete your default en0 gateway.
4. After running the client code, a new network tunnel is created with the name `utun13`. This tunnel needs to be added to the routes on your machine network table. To do so, you need to do the following step:
    a. Run command `sudo ifconfig utun13 inet 10.0.0.1 10.0.0.2 up`
        - `10.0.0.1` is your local machine
        - `10.0.0.2` is the IP of this application, and set the flag up so it's up and running
    b. Run command `sudo route add 0.0.0.0/0 -interface utun13`
        - we're saying whatever data/packets received from or sent to your machine would directly go into this Go client app
        - basically, you have rerouted the traffic of your machine to go through this app

## To revert to default network settings
1. Stop the client Go app.
2. Run `sudo route add default 000.000.0.000` (000.000.0.000 is the address unique to you and your Mac machine)