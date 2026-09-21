import {LngLat, Map as MapLibreMap} from 'maplibre-gl';
import {circle as turfCircle} from '@turf/circle';

/**
 * Options for creating a Circle.
 */
export type CircleOptions = {
    /** Color of the circle fill and outline. */
    color?: string;
    /** Radius of the circle in meters. */
    radius?: number;
    /** Center of the circle as a LngLat object. */
    center?: LngLat;
};

/**
 * Represents a circle on a MapLibre map.
 */
export class Circle {
    /**
     * The radius of the circle in meters.
     * @private
     * @type {number} The radius of the circle in meters.
     */
    private radius: number = 100;

    /**
     * The center of the circle as a LngLat object.
     * @private
     * @type {LngLat} The center of the circle as a LngLat object.
     */
    private center: LngLat = new LngLat(0, 0);

    /**
     * The MapLibre map instance.
     * @private
     * @type {MapLibreMap | null} The MapLibre map instance.
     */
    private map: MapLibreMap | null = null;

    /**
     * The unique identifier for the circle.
     * @private
     * @type {number} The unique identifier for the circle.
     */

    private id: number = Math.random() * 1000000;
    /**
     * The color of the circle fill and outline.
     * @private
     * @type {string} The color of the circle fill and outline.
     */
    private color: string = '#0000FF';

    /**
     * Creates a new Circle instance.
     * @param options Optional configuration for the circle.
     */
    constructor(options?: CircleOptions) {
        if (options) {
            if (options.color) {
                this.color = options.color;
            }
            if (options.radius) {
                this.radius = options.radius;
            }
            if (options.center) {
                this.center = options.center;
            }
        }
    }

    /**
     * Gets the radius of the circle in meters.
     * @returns The radius of the circle.
     */
    getRadius(): number {
        return this.radius;
    }

    /**
     * Sets the radius of the circle and updates it on the map.
     * @param radius The new radius in meters.
     */
    setRadius(radius: number): void {
        this.radius = radius;

        this.updateOnMap();
    }

    /**
     * Sets the center of the circle and updates it on the map.
     * @param latlng The new center as a LngLat object.
     */
    setLatLng(latlng: LngLat) {
        this.center = latlng;
        this.updateOnMap();
    }

    /**
     * Gets the center of the circle.
     * @returns The center as a LngLat object.
     */
    getLatLng(): LngLat {
        return this.center;
    }

    /**
     * Adds the circle to the specified MapLibre map.
     * @param map The MapLibre map instance.
     */
    addTo(map: MapLibreMap) {
        this.map = map;
        this.updateOnMap();
    }

    /**
     * Updates the circle's representation on the map.
     * Adds or updates the GeoJSON source and layers for the circle.
     * @private
     */
    private updateOnMap() {
        if (this.map && this.center && this.radius > 0) {
            const circle = turfCircle(this.center.toArray(), this.radius, {units: 'meters'});
            if (this.map.getSource('location-radius-' + this.id)) {
                // @ts-ignore
                this.map.getSource('location-radius-' + this.id).setData(circle);
            } else {
                this.map.addSource('location-radius-' + this.id, {
                    type: 'geojson',
                    data: circle,
                });
            }

            if (!this.map.getLayer('location-radius-outline-' + this.id)) {
                this.map.addLayer({
                    id: 'location-radius-' + this.id,
                    type: 'fill',
                    source: 'location-radius-' + this.id,
                    paint: {
                        'fill-color': this.color,
                        'fill-opacity': 0.2,
                    },
                });

                // Add a line layer to draw the circle outline
                this.map.addLayer({
                    id: 'location-radius-outline-' + this.id,
                    type: 'line',
                    source: 'location-radius-' + this.id,
                    paint: {
                        'line-color': this.color,
                        'line-width': 3,
                    },
                });
            }
        }
    }
}
