# Binance diff-dept-stream

Binance offers a websocket stream allowing the user to update a local order book
with the latest value of a token.

## Overview

The application entails three distinct steps: 1- Read and parse data from the
websocket stream. 2- Process data. 3- Print data to standard out.

Each of these steps will be separate go-routines running, with data passed
between each go-routine, using a mutex to ensure thread safety.

## Parallel processing

Running each step in its own go-routine ensure no step will keep another from
processing, given the availability of a thread to run the process.

### Fetching data

Use a client interface with the endpoints implemented as methods for fetching
required data.

A structure of the client offers dependency injection for the websocket and http
client, ensuring the program can easily be tested.
