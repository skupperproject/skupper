"""
Discovers a free TCP port to be used by the normal
listener defined in the router config file (json).
The port will be bound to 127.0.0.1 only and discovery
starts at port 5671.
Once a free port is discovered, the provided router
config file will be updated accordingly.
"""
import json
import os
import socket
import sys


def next_free_port():
    """
    Discovers the next free TCP port (starting from 5671)
    """
    s = socket.socket()
    port=5671
    while port <= 65535:
        try:
            s.bind(('127.0.0.1', port))
            break
        except:
            port+=1
    if port > 65535:
        raise Exception("Unable to determine free TCP port to use")
    return port


def usage():
    """
    Shows usage and exit with return code 1
    """
    print("Usage: %s skrouterd.json" %(sys.argv[0]))
    sys.exit(1)


def main():
    """
    Validates arguments and run the port discovery and update procedures
    """
    if len(sys.argv) != 2:
        usage()
    config_file = sys.argv[1]
    if not os.path.isfile(config_file):
        print("Invalid router config file: %s" %(config_file))
        usage()

    data = {}
    with open(config_file) as f:
        data = json.load(f)

    for entry in data:
        if entry[0] != 'listener':
            continue
        listener = entry[1]
        if listener['role'] == 'normal':
            free_port = next_free_port()
            listener['port'] = free_port

    f.close()

    with open(config_file, "w") as f:
        json.dump(data, f, indent=4)


# Identify normal listener and update port
main()
