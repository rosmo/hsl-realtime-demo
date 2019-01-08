# Copyright 2018 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import datetime
import redis
import os
import re
import sys

from flask import Flask, jsonify, json, make_response, render_template, send_from_directory

app = Flask(__name__)

@app.route('/healthz', methods=['GET','POST','OPTIONS'])
def health():
    resp = make_response("OK!", 200)
    return resp
    
@app.route('/jsondata', methods=['GET','POST','OPTIONS'])
def jsondata():
    r = redis.Redis(host=os.environ.get('REDIS_HOST', 'localhost'), port=int(os.environ.get('REDIS_PORT', 6379)), db=0)
    keys = r.keys('*')
    result = {}
    for key in keys: 
        result[key.decode('utf-8')] = []
        content = r.lrange(key, 0, 3)
        for c in content:
            decoded = json.loads(c)
            res = []
            for k in ['route_number', 'location', 'speed', 'direction', 'heading']:
                if k in decoded:
                    if k == 'location':
                        location = decoded[k][6:-1].split(' ')
                        res.append(list(map(float, location)))
                    else:
                        res.append(decoded[k])
            if len(res) == 5:
                result[key.decode('utf-8')].append(res)

    resp = make_response(jsonify(result), 200)
    resp.headers['Access-Control-Allow-Origin'] = '*'
    resp.headers['Access-Control-Allow-Headers'] = 'Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With'
    resp.headers['Content-Type'] = 'application/json'
    return resp

@app.route('/css/<path:path>')
def send_css(path):
    return send_from_directory('static/css', path)

@app.route('/images/<path:path>')
def send_images(path):
    return send_from_directory('static/images', path)

@app.route('/js/<path:path>')
def send_js(path):
    return send_from_directory('static/js', path)

@app.route('/js/index.js')
def send_index_js():
    tpl = render_template('index.js', url=os.environ.get('FRONTEND_URL', 'SET-YOUR-FRONTEND-URL!'))
    resp = make_response(tpl, 200)
    resp.headers['Content-Type'] = 'text/javascript'
    return resp

@app.route('/', methods=['GET','POST','OPTIONS'])
def root():
    return render_template('index.html', apikey=os.environ.get('MAPS_APIKEY', 'SET-YOUR-API-KEY!'))

if __name__ == '__main__':
    # This is used when running locally only. When deploying to Google App
    # Engine, a webserver process such as Gunicorn will serve the app. This
    # can be configured by adding an `entrypoint` to app.yaml.
    # Flask's development server will automatically serve static files in
    # the "static" directory. See:
    # http://flask.pocoo.org/docs/1.0/quickstart/#static-files. Once deployed,
    # App Engine itself will serve those files as configured in app.yaml.
    app.run(host='0.0.0.0', port=sys.argv[1], debug=True)
