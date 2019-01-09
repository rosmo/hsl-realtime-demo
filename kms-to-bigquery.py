#!/usr/bin/python
#
# Converts KML files to BigQuery WKT geography objects (CSV)

from __future__ import print_function
import sys
import re
import xml.etree.ElementTree

e = xml.etree.ElementTree.parse(sys.argv[1]).getroot()

p = re.compile('^\{(.+?)\}')
matches = p.match(e.tag)
xmlns = matches.group(1)

document = e[0]

for pmark in document.findall('.//xmlns:Placemark', { 'xmlns' : xmlns }):
    name = pmark.find('./xmlns:name', { 'xmlns' : xmlns })
    print('"%s",' % name.text, end='')
    for coord in pmark.findall('.//xmlns:coordinates', { 'xmlns' : xmlns }):
        coordmatches = re.findall('(\d+\.\d+,\d+\.\d+),0[ \n]', coord.text)
        print('"POLYGON((', end='')
        i = 0
        for m in coordmatches:
            if i > 0:
                print(',', end='')
            print(m.replace(',', ' '), end='')
            i += 1

        print('))"')

        
