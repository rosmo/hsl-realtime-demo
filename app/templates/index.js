/*
 * Copyright 2017 Google Inc. All rights reserved.
 *
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
 * file except in compliance with the License. You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
 * ANY KIND, either express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

// @ts-check
/// <reference path="../typings/index.d.ts"/>

/* eslint-disable no-unused-vars, no-shadow-global */
/* globals google firebase */

const mapStyle = [
  {
    elementType: 'geometry',
    stylers: [{color: '#eceff1'}]
  },
  {
    elementType: 'labels',
    stylers: [{visibility: 'off'}]
  },
  {
    featureType: 'administrative',
    elementType: 'labels',
    stylers: [{visibility: 'on'}]
  },
  {
    featureType: 'road',
    elementType: 'geometry',
    stylers: [{color: '#cfd8dc'}]
  },
  {
    featureType: 'road',
    elementType: 'geometry.stroke',
    stylers: [{visibility: 'off'}]
  },
  {
    featureType: 'road.local',
    stylers: [{visibility: 'off'}]
  },
  {
    featureType: 'water',
    stylers: [{color: '#b0bec5'}]
  }
];

class MarkerManager {
  constructor(map) {
    this.map = map;
    this.markers = [];
  }

  add(location, icon, title) {
    const marker = new google.maps.Marker({
      position: location,
      map: this.map,
      icon: icon,
      title: title,
      optimized: false
    });
    this.markers.push(marker);
  }

  clear() {
    this.markers.forEach(marker => {
      marker.setMap(null);
    });
    this.markers.length = 0;
  }
}

function geocodeAddress(address, map, icon, title) {
  const geocoder = new google.maps.Geocoder();
  geocoder.geocode({address: address}, (results, status) => {
    if (status === 'OK') {
      const marker = new google.maps.Marker({
        map: map,
        position: results[0].geometry.location,
        icon: icon,
        title: title,
        optimized: false
      });
    } else {
      console.log(
        'Geocode was not successful for the following reason: ' + status
      );
    }
  });
}

function initMap() {
  var helsinki = {lat: 60.179, lng: 24.938};
  const map = new google.maps.Map(document.getElementById('map'), {
    disableDefaultUI: false,
    zoom: 13,
    center: helsinki
  });

  const markerManager = new MarkerManager(map);
  const cardsElement = document.getElementsByClassName('cards')[0];
  const displayTimeElement = document.getElementsByClassName('display-time')[0];
  const pageMarkerPanelElts = [
    document.getElementById('page-marker-panel-0'),
    document.getElementById('page-marker-panel-1'),
    document.getElementById('page-marker-panel-2')
  ];

  const busLocationMarkers = {};

  setInterval(function () {
    $.ajax('{{ url }}/jsondata').done(function (resp) {

      for (let key in busLocationMarkers) {
        if (resp === null || !(key in resp)) {
          const marker = busLocationMarkers[key];
          marker.setMap(null);
          delete busLocationMarkers[key];
        }
      }
  
      for (let key in resp) {
        const bus = resp[key];
        
        if (bus.length > 0 && key in busLocationMarkers) {
          const marker = busLocationMarkers[key];
          marker.setPosition({
            lat: bus[0][1][1],
            lng: bus[0][1][0]
          });
        } else if (bus.length > 0) {
          const url = 'images/busmarker_8.png';
          const marker = new google.maps.Marker({
            position: {
              lat: bus[0][1][1],
              lng: bus[0][1][0]
            },
            map: map,
            icon: {
              url,
              anchor: new google.maps.Point(4, 5)
            },
            title: bus[0][0],
            optimized: false,
            duration: 2000,
            draggable: false,
            zIndex: 99999
          });
          
          var infowindow = new google.maps.InfoWindow({
            content: '<p>Route: ' + bus[0][0] + '</p><p>Speed: ' + bus[0][2] + ' km/h</p>'
          });
          google.maps.event.addListener(marker, 'click', function() {
            infowindow.open(map, marker);
          });
          busLocationMarkers[key] = marker;
        }
      }

    });

  }, 2500);

  
}
