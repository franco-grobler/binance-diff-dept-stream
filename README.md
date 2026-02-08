# Binance diff-dept-stream

Binance offers a websocket stream allowing the user to update a local order book
with the latest value of a token.

## Overview

In this assessment, you’ll be creating a Go application which interacts with
Binance’s WebSocket API. Your solution should be containerized using Docker.
Please also provide a README that explains how to set up and run the
application, any design decisions you made, performance considerations, and any
limitations of your design. Use idiomatic Go standards and project layouts where
possible.

### Tasks

1. Connect to Binance’s WebSocket API and subscribe to the DiM. Depth Stream.
   - [Documentation can be found here.](https://developers.binance.com/docs/binance-spot-api-docs/web-socket-streams#diM-depth-stream)
   - Process incoming data on multiple symbols, using appropriate data
     structures to maintain all orderbook levels in memory. Use the standard
     library JSON package (V1) to perform the unmarshalling.
   - On each update, print the top level prices to stdout in the following
     format:
     `“{symbol”:”BTCUSDT”,“ask”:100001.00,”bid”:100000.00 “ts”:1672515782136}”`

2. Propose alternate strategies for the unmarshalling step above in order to
   improve performance.
   - Your strategies should consider: heap allocations and speed.
   - Provide verification via testing and benchmarks.

## Design

The application entails three distinct steps: 1- Read and parse data from the
websocket stream. 2- Process data. 3- Print data to standard out.

Each of these steps will be separate go-routines running, with data passed
between each go-routine, using a mutex to ensure thread safety.

### Parallel processing

Running each step in its own go-routine ensure no step will keep another from
processing, given the availability of a thread to run the process.

#### Fetching data

Use a client interface with the endpoints implemented as methods for fetching
required data.

A structure of the client offers dependency injection for the websocket and http
client, ensuring the program can easily be tested.
