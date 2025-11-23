/**
 * @fileoverview uihelpers - UI utility helpers for antenna and map operations.
 * Contains: UIHelpers singleton class, azimuth calculation, collision point detection, Friis signal calculation, and event-driven UI updates.
 */

import {DataProvider, DataProviderEventEnum} from "./DataProvider";
import {LngLat} from "maplibre-gl";
import {AntennaEnum} from "./Antenna";
import {ChartController} from "./controllers/ChartController";
import {DistanceUIController} from "./controllers/DistanceUIController";
import {LineController} from "./controllers/LineController";

/**
 * Singleton class for UI helper methods and event-driven updates related to antennas and map.
 */
export class UIHelpers {
    /**
     * The singleton instance of UIHelpers.
     * @type {UIHelpers | null}
     */
    private static instance: UIHelpers | null = null;

    /**
     * Private constructor to enforce singleton pattern.
     */
    private constructor() {
    }

    /**
     * Initializes UIHelpers by subscribing to antenna position change events.
     * @returns {void}
     */
    public init(): void {
        DataProvider.getSharedInstance().addEventListener(DataProviderEventEnum.ANTENNA_POSITION_CHANGED, this.updatedPositionHandler);
    }

    /**
     * Returns the singleton instance of UIHelpers.
     * @returns {UIHelpers} The singleton instance
     */
    static getInstance(): UIHelpers {
        if (!UIHelpers.instance) {
            UIHelpers.instance = new UIHelpers();
        }
        return UIHelpers.instance;
    }

    /**
     * Calculates the azimuth (bearing) in degrees from coord1 to coord2.
     * @param {LngLat} coord1 - Starting point
     * @param {LngLat} coord2 - Ending point
     * @returns {number} Azimuth in degrees (0° = North)
     */
    static calculateAzimuth(coord1: LngLat, coord2: LngLat): number | null {
        if (!coord1 || !coord2) {
            return null;
        }
        const toRad = (deg: number) => deg * Math.PI / 180;
        const toDeg = (rad: number) => rad * 180 / Math.PI;
        const lat1 = toRad(coord1.lat);
        const lat2 = toRad(coord2.lat);
        const dLon = toRad(coord2.lng - coord1.lng);
        const y = Math.sin(dLon) * Math.cos(lat2);
        const x = Math.cos(lat1) * Math.sin(lat2) - Math.sin(lat1) * Math.cos(lat2) * Math.cos(dLon);
        let brng = Math.atan2(y, x);
        brng = toDeg(brng);
        return (brng + 360) % 360;
    }

    /**
     * Returns points from data where the y value exceeds a threshold function.
     * @param {[number, number][]} data - Array of [x, y] points
     * @param {function} threshold - Function returning max y for a given x
     * @returns {[number, number][]} Array of [x, y] collision points
     */
    static getCollisionPoints(data: [number, number][], threshold: (x) => number): [number, number][] {
        let points: [number, number][] = [];

        let last_collision_index: number = -10;
        for (let i = 0; i < data.length; i++) {
            let point = data[i];
            let x = point[0];
            let y = point[1];
            let max_y = threshold(x);
            if (max_y < y) {
                if (last_collision_index !== (i - 1)) {
                    points.push([x, max_y]);
                }
                last_collision_index = i;
            } else if (last_collision_index == (i - 1)) {
                points.push([data[i-1][0], max_y]);
            }
        }
        return points;
    }


    /**
     * Calculates the Friis transmission loss for a given distance, frequency, and gain.
     * @param distance - Distance in meters
     * @param frequency - Frequency in Hz
     * @param gainTx_dBi - Transmitter gain in dBi
     * @param gainRx_dBi - Receiver gain in dBi
     * @returns {number} Friis transmission loss in dB
     */
    static friisPropagationLoss(distance: number, frequency: number, gainTx_dBi: number = 0, gainRx_dBi: number = 0): number {
        const c = 299792458; // Speed of light (m/s)
        const wavelength = c / frequency;

        // Convert dBi to linear
        const gainTx = Math.pow(10, gainTx_dBi / 10);
        const gainRx = Math.pow(10, gainRx_dBi / 10);

        // Friis transmission equation (linear ratio)
        const ratio = (gainTx * gainRx * Math.pow(wavelength, 2)) / Math.pow(4 * Math.PI * distance, 2);

        // Convert ratio to dB
        if (10 * Math.log10(ratio) < 0) {
            return 10 * Math.log10(ratio);
        } else {
            return 0;
        }
    }

    /**
     * Calculates the Friis propagation distance for a given frequency, gain, and maximum loss.
     * @param frequency - Frequency in Hz
     * @param gainTx_dBi - Transmitter gain in dBi
     * @param gainRx_dBi - Receiver gain in dBi
     * @param loss_dB - Maximum loss in dB
     * @returns {number} Friis propagation distance in meters
     */
    static friisPropagationDistance(loss_dB: number, frequency: number, gainTx_dBi: number = 0, gainRx_dBi: number = 0): number {
        const c = 299792458; // Speed of light (m/s)
        const wavelength = c / frequency;

        // Convert dBi to linear
        const gainTx = Math.pow(10, gainTx_dBi / 10);
        const gainRx = Math.pow(10, gainRx_dBi / 10);

        // Convert loss from dB to linear ratio
        const loss_linear = Math.pow(10, loss_dB / 10);

        // Rearranged Friis equation:
        // loss_linear = (gainTx * gainRx * wavelength^2) / (4 * pi * d)^2
        // => (gainTx * gainRx * wavelength^2) / loss_linear = (4 * pi * d)^2
        // => sqrt((gainTx * gainRx * wavelength^2) / loss_linear) / (4 * pi) = d
        const numerator = gainTx * gainRx * Math.pow(wavelength, 2);
        return Math.sqrt(numerator / loss_linear) / (4 * Math.PI);

    }


    /**
     * Calculates the received signal strength (in dBm) using the Friis transmission equation.
     * @param {number} pt_dBm - Transmitted power in dBm
     * @param {number} gt_dBi - Transmitter antenna gain in dBi
     * @param {number} gr_dBi - Receiver antenna gain in dBi
     * @param {number} freq_Hz - Frequency in Hz
     * @param {number} distance_m - Distance between antennas in meters
     * @returns {number} Received signal strength in dBm
     */
    static calculateFriisSignal(pt_dBm: number, gt_dBi: number, gr_dBi: number, freq_Hz: number, distance_m: number): number {
        const pathLoss_dB = this.friisPropagationLoss(distance_m, freq_Hz, gt_dBi, gr_dBi);
        return pt_dBm + gt_dBi + gr_dBi + pathLoss_dB;
    }

    static getMaxDistanceFriis(pt_dBm: number, gt_dBi: number, gr_dBi: number, freq_Hz: number, sensitivity_dBm: number): number {
        const combinded_gain = gt_dBi + gr_dBi + pt_dBm;
        if (combinded_gain > sensitivity_dBm) {
            const max_pathloss_dB = sensitivity_dBm - combinded_gain;
            return this.friisPropagationDistance(max_pathloss_dB, freq_Hz, gt_dBi, gr_dBi);
        }
        return -1;
    }

    /**
     * Handles updates when antenna positions change: updates line, chart, and distance UI, and recalculates azimuths.
     * @returns {void}
     */
    private updatedPositionHandler(): void {
        LineController.getInstance().update();
        ChartController.getInstance().update();
        DistanceUIController.getInstance().update();
        let antenna_one = DataProvider.getSharedInstance().getAntenna(AntennaEnum.ONE);
        let antenna_two = DataProvider.getSharedInstance().getAntenna(AntennaEnum.TWO);
        if (antenna_one && antenna_two) {
            antenna_one.setAzimuth(antenna_two);
            antenna_two.setAzimuth(antenna_one);
        }
    }
}
