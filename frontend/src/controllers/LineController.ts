/**
 * LineController.ts - Handles drawing and updating the line between antennas on the map
 * Contains: LineController singleton for managing the line animation layer and geojson source
 */

import {Map as MapLibreMap} from 'maplibre-gl';
import {Antenna, AntennaEnum} from '../Antenna';
import {DataProvider} from '../DataProvider';
import {DistanceUIController} from './DistanceUIController';

/**
 * Singleton controller for drawing and updating the line between antennas on the map.
 */
export class LineController {
    private static instance: LineController | null = null;

    /**
     * Private constructor for singleton pattern.
     */
    private constructor() {}

    /**
     * Returns the singleton instance of LineController.
     * @returns {LineController} The singleton instance
     */
    static getInstance(): LineController {
        if (!LineController.instance) {
            LineController.instance = new LineController();
        }
        return LineController.instance;
    }

    /**
     * GeoJSON object for the line between antennas.
     * @type {Object}
     */
    geojson: any = {
        type: 'FeatureCollection',
        features: [
            {
                type: 'Feature',
                geometry: {
                    type: 'LineString',
                    coordinates: [[0, 0]],
                },
            },
        ],
    };

    /**
     * Draws or updates the line between two antennas on the map.
     * @param {MapLibreMap | null} map - The map instance
     * @param {Antenna | null} antenna_one - First antenna
     * @param {Antenna | null} antenna_two - Second antenna
     * @returns {void}
     */
    createLineBetweenAntennas(
        map: MapLibreMap | null = null,
        antenna_one: Antenna | null = null,
        antenna_two: Antenna | null = null,
    ): void {
        if (!antenna_one || !antenna_two || !antenna_one.getLatlng() || !antenna_two.getLatlng()) {
            if (map.getLayer('line-animation')) {
                map.removeLayer('line-animation');
            }
            return;
        }
        if (!map.getSource('line')) {
            map.addSource('line', {
                type: 'geojson',
                data: this.geojson,
            });
        }
        map.removeLayer('line-animation');

        map.addLayer({
            id: 'line-animation',
            type: 'line',
            source: 'line',
            layout: {
                'line-cap': 'round',
                'line-join': 'round',
            },
            paint: {
                'line-color': this.getLineColor(antenna_one, antenna_two),
                'line-width': 3,
                'line-opacity': 0.8,
            },
        });

        this.geojson.features[0].geometry.coordinates = [
            [antenna_one.getLatlng().lng, antenna_one.getLatlng().lat],
            [antenna_two.getLatlng().lng, antenna_two.getLatlng().lat],
        ];
        // @ts-ignore
        map.getSource('line').setData(this.geojson);
    }

    private getLineColor(antenna_one: Antenna, antenna_two: Antenna): string {
        const distance = DistanceUIController.calculateDistanceBetweenAntennas(antenna_one, antenna_two);
        if (antenna_one.getExpectedRange() > distance && antenna_two.getExpectedRange() > distance) {
            return 'green';
        } else if (antenna_one.getExpectedRange() > distance && antenna_two.getExpectedRange() <= distance) {
            return 'orange';
        } else if (antenna_one.getExpectedRange() <= distance && antenna_two.getExpectedRange() > distance) {
            return 'orange';
        }

        return 'red';
    }

    /**
     * Updates the line between antennas using the current map and antenna positions.
     * @returns {void}
     */
    update(): void {
        let antenna_one = DataProvider.getSharedInstance().getAntenna(AntennaEnum.ONE);
        let antenna_two = DataProvider.getSharedInstance().getAntenna(AntennaEnum.TWO);
        let map = DataProvider.getSharedInstance().getMap();
        this.createLineBetweenAntennas(map, antenna_one, antenna_two);
    }
}
