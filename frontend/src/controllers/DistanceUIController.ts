/**
 * DistanceUIController.ts - Handles UI updates for displaying the distance between antennas
 * Contains: DistanceUIController singleton for calculating and updating the distance display
 */

import {Antenna, AntennaEnum} from '../Antenna';
import {DataProvider} from '../DataProvider';

/**
 * Singleton controller for updating the UI with the distance between antennas.
 */
export class DistanceUIController {
    private static instance: DistanceUIController | null = null;

    /**
     * Private constructor for singleton pattern.
     */
    private constructor() {}

    /**
     * Returns the singleton instance of DistanceUIController.
     * @returns {DistanceUIController} The singleton instance
     */
    static getInstance(): DistanceUIController {
        if (!DistanceUIController.instance) {
            DistanceUIController.instance = new DistanceUIController();
        }
        return DistanceUIController.instance;
    }

    /**
     * Calculates the distance in meters between two antennas using the Haversine formula.
     * @param {Antenna} antenna_one - First antenna
     * @param {Antenna} antenna_two - Second antenna
     * @returns {number} Distance in meters
     */
    public static calculateDistanceBetweenAntennas(antenna_one: Antenna, antenna_two: Antenna): number {
        if (!antenna_one || !antenna_two || !antenna_one.getLatlng() || !antenna_two.getLatlng()) {
            return -1;
        }
        const coord1 = antenna_one.getLatlng();
        const coord2 = antenna_two.getLatlng();
        const R = 6371000; // Earth radius in meters
        const toRad = (deg: number) => (deg * Math.PI) / 180;
        const dLat = toRad(coord2.lat - coord1.lat);
        const dLon = toRad(coord2.lng - coord1.lng);
        const lat1 = toRad(coord1.lat);
        const lat2 = toRad(coord2.lat);
        const a =
            Math.sin(dLat / 2) * Math.sin(dLat / 2) +
            Math.sin(dLon / 2) * Math.sin(dLon / 2) * Math.cos(lat1) * Math.cos(lat2);
        const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
        const distance = R * c;
        return distance;
    }

    /**
     * Updates the UI with the current distance between antennas.
     * @returns {void}
     */
    update(): void {
        let antenna_one = DataProvider.getSharedInstance().getAntenna(AntennaEnum.ONE);
        let antenna_two = DataProvider.getSharedInstance().getAntenna(AntennaEnum.TWO);
        if (!antenna_one || !antenna_two) {
            document.querySelector('.distance-row span').textContent = 'N/A m';
            return;
        }
        let distance = DistanceUIController.calculateDistanceBetweenAntennas(antenna_one, antenna_two);
        document.querySelector('.distance-row span').textContent = distance.toFixed() + ' m';
    }
}
