from __future__ import print_function
import sys
import os

try:
	filename = sys.argv[1]
	is_file = os.path.isfile(filename)
	if not is_file:
		raise Exception()
except Exception as e:
	print ("Usage: python3 expandvars.py <absolute_file_path>. Example - python3 expandvars.py /tmp/skrouterd.conf")
	## Unix programs generally use 2 for command line syntax errors
	sys.exit(2)

out_list = []
with open(filename) as f:
	for line in f:
		if line.startswith("#") or not '$' in line:
			out_list.append(line)
		else:
			out_list.append(os.path.expandvars(line))

with open(filename, 'w') as f:
	for out in out_list:
		f.write(out)
