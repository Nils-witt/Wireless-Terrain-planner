/**
 * @fileoverview Antenna - Antenna model and logic for position, height, gain, power, and marker management.
 * Contains: AntennaEnum, Antenna class for antenna state and map marker updates.
 */

import {LngLat, Map as MapLibreMap, Marker} from 'maplibre-gl';
import {DataProvider, DataProviderEventEnum} from './DataProvider';
import {UIHelpers} from './uihelpers';
import {DistanceUIController} from './controllers/DistanceUIController';
import {Circle} from './Enitities/Circle';

/**
 * Enum for identifying antennas.
 * @readonly
 * @enum {string}
 */
export enum AntennaEnum {
    ONE = 'left',
    TWO = 'right',
}

/**
 * Class representing an antenna, including position, height, and marker logic.
 */
export class Antenna {
    /**
     * Height over ground in meters.
     * @type {number | undefined}
     */
    private height: number | undefined = undefined;
    /**
     * Latitude and Longitude in degrees.
     * @type {LngLat | undefined}
     */
    private latlng: LngLat | undefined = undefined;
    /**
     * Transmitted power in dBm
     */
    private transmittedPower_dBm: number = 0;
    /**
     * Antenna gain in dBi
     */
    private antennaGain_dBi: number = 0;
    /**
     * The marker for the antenna on the map.
     * @type {Marker}
     */
    private marker: Marker;

    /**
     * The circle for the antenna on the map.
     * @private
     * @type {Circle}
     */
    private circle: Circle;
    /**
     * The map instance.
     * @type {MapLibreMap | null}
     */
    private map: MapLibreMap | null = null;
    /**
     * The antenna identifier.
     * @type {AntennaEnum}
     */
    private id: AntennaEnum;

    /**
     * The antenna sensitivity in dBm.
     * @type {number}
     */
    private antennaSensitivity: number = 0;

    /**
     * The associated antenna. We can associate an antenna with another antenna to calculate the expected signal.
     * @type {Antenna | null}
     */
    private associatedAntenna: Antenna | null = null;

    /**
     * Frequency in Hz
     * @private
     */
    private frequency: number = 0;

    /**
     * Constructs an Antenna instance.
     * @param {AntennaEnum} id - Identifier for the antenna
     */
    constructor(id: AntennaEnum) {
        this.id = id;
        if (this.id === AntennaEnum.ONE) {
            this.marker = new Marker({
                color: '#0000FF',
            });
            this.circle = new Circle({
                color: '#0000FF',
            });
        }
        if (this.id === AntennaEnum.TWO) {
            this.marker = new Marker({
                color: 'magenta',
            });
            this.circle = new Circle({
                color: 'magenta',
            });
        }
    }

    /**
     * Gets the antenna's identifier.
     * @returns {AntennaEnum} The antenna's identifier
     */
    getId(): AntennaEnum {
        return this.id;
    }

    /**
     * Sets the antenna's identifier.
     * @param {AntennaEnum} id - The new identifier
     * @returns {void}
     */
    setId(id: AntennaEnum): void {
        this.id = id;
    }

    /**
     * Gets the antenna's height.
     * @returns {number | undefined} The antenna's height
     */
    getHeight(): number | undefined {
        return this.height;
    }

    /**
     * Gets the antenna's position.
     * @returns {LngLat | undefined} The antenna's position
     */
    getLatlng(): LngLat | undefined {
        return this.latlng;
    }

    /**
     * Gets the transmitted power in dBm.
     * @returns {number} Transmitted power in dBm
     */
    getTransmittedPower(): number {
        return this.transmittedPower_dBm;
    }

    /**
     * Gets the antenna gain in dBi.
     * @returns {number} Antenna gain in dBi
     */
    getAntennaGain(): number {
        return this.antennaGain_dBi;
    }

    /**
     * Sets the antenna's height and updates the UI.
     * @param {number | undefined} height - The new height
     * @returns {void}
     */
    setHeight(height: number | undefined): void {
        this.height = height;
        this.updateUIElement('Height', height);
        this.updateExpectedSignal();
    }

    /**
     * Sets the transmitted power in dBm.
     * @param {number} power - Transmitted power in dBm
     * @returns {void}
     */
    setTransmittedPower(power: number): void {
        this.transmittedPower_dBm = power;
        this.updateUIElement('Transmitted Power (dBm)', power);
        this.updateExpectedSignal();
    }

    /**
     * Sets the antenna gain in dBi.
     * @param {number} gain - Antenna gain in dBi
     * @returns {void}
     */
    setAntennaGain(gain: number): void {
        this.antennaGain_dBi = gain;
        this.updateUIElement('Antenna Gain (dBi)', gain);
        this.updateExpectedSignal();
    }

    /**
     * Sets the antenna's position and updates the marker if a map is set.
     * @param {LngLat | undefined} latlng - The new position
     * @returns {void}
     */
    setLatlng(latlng: LngLat | undefined): void {
        this.latlng = latlng;

        if (this.map) {
            this.marker.setLngLat(this.latlng);
            this.marker.addTo(this.map);
            this.circle.setLatLng(latlng);
            this.circle.addTo(this.map);
        }
        const selector = `.station-card.${this.id} .station-location .value`;
        document.querySelector(selector).textContent = latlng.lat.toPrecision(4) + ', ' + latlng.lng.toPrecision(4);
        DataProvider.getSharedInstance().emitEvent(DataProviderEventEnum.ANTENNA_POSITION_CHANGED, {
            antenna: this,
            antenna_id: this.id,
        });
        this.updateExpectedSignal();
    }

    /**
     * Adds the antenna marker to the map.
     * @param {MapLibreMap} map - The map instance
     * @returns {void}
     */
    public addToMap(map: MapLibreMap): void {
        this.map = map;
        if (this.latlng) {
            this.marker.setLngLat(this.latlng);
            this.marker.addTo(map);
        }
    }

    /**
     * Removes the antenna marker from the map.
     * @returns {void}
     */
    public removeFromMap(): void {
        if (this.map) {
            this.marker.remove();
            this.map = null;
        }
    }

    /**
     * Sets the device name in the UI.
     * @param {string} name - The device name
     * @returns {void}
     */
    setDeviceName(name: string): void {
        const selector = `.station-card.${this.id} .station-title .value`;
        document.querySelector(selector).textContent = name;
    }

    /**
     * Sets the frequency in the UI.
     * @param {number} frequency - The frequency value
     * @returns {void}
     */
    setFrequency(frequency: number): void {
        this.frequency = frequency;
        this.updateUIElement('Frequency (MHz)', String(frequency / 1000));
        this.updateExpectedSignal();
    }

    /**
     * Sets the azimuth angle to another antenna.
     * @param {Antenna} to - The other antenna
     * @returns {void}
     */
    setAzimuth(to: Antenna): void {
        const azimuth = UIHelpers.calculateAzimuth(this.getLatlng(), to.getLatlng());
        this.updateUIElement('Azimuth', azimuth ? azimuth.toFixed(1) : 'N/A');
    }

    /**
     * Sets the expected signal value in the UI.
     * @param {number} signal - The expected signal
     * @returns {void}
     */
    setExpectedSignal(signal: number): void {
        const selector = `.station-card.${this.id} .expected-signal .value`;
        document.querySelector(selector).textContent = signal.toFixed(2) + ' dBm';
    }

    /**
     * Gets the expected range value based on the antenna parameters.
     * @returns {number} The expected range in meters
     */
    getExpectedRange(): number {
        if (this.associatedAntenna) {
            return UIHelpers.getMaxDistanceFriis(
                this.transmittedPower_dBm,
                this.antennaGain_dBi,
                this.associatedAntenna.getAntennaGain(),
                this.getFrequencyHz(),
                this.associatedAntenna.getSensitivity(),
            );
        } else {
            return UIHelpers.getMaxDistanceFriis(
                this.transmittedPower_dBm,
                this.antennaGain_dBi,
                this.antennaGain_dBi,
                this.getFrequencyHz(),
                this.antennaSensitivity,
            );
        }
    }

    /**
     * Sets the associated antenna for the object.
     *
     * @param {Antenna | null} antenna - The antenna to associate with the object, or null to disassociate any currently associated antenna.
     * @return {void}
     */
    setAssociatedAntenna(antenna: Antenna | null): void {
        this.associatedAntenna = antenna;
        this.updateExpectedSignal();
    }

    getAssociatedAntenna(): Antenna | null {
        return this.associatedAntenna;
    }

    setSensitivity(sensitivity: number): void {
        this.antennaSensitivity = sensitivity;
        this.updateUIElement('Sensitivity', sensitivity);
    }

    private updateUIElement(label: string, value: number | string): void {
        document.querySelectorAll(`.station-card.${this.id} .station-row`).forEach((row) => {
            row.querySelectorAll('.label').forEach((labelEl, idx) => {
                if (labelEl.textContent.trim() === label) {
                    let el = row.querySelectorAll('.value')[idx] as HTMLInputElement;
                    el.value = String(value);
                }
            });
        });
    }

    getSensitivity(): number {
        return this.antennaSensitivity;
    }

    /**
     * Returns the frequency in Hz. The frequency is stored in kHz (see setFrequency).
     * @returns {number} The frequency in Hz
     */
    private getFrequencyHz(): number {
        return this.frequency * 1000;
    }

    updateExpectedSignal(no_rec = false): void {
        if (!this.associatedAntenna) {
            return;
        }
        const distance = DistanceUIController.calculateDistanceBetweenAntennas(this, this.associatedAntenna);
        const pr = UIHelpers.calculateFriisSignal(
            this.getAssociatedAntenna().getTransmittedPower(),
            this.getAntennaGain(),
            this.getAssociatedAntenna().getAntennaGain(),
            // The associated antenna transmits, so its frequency applies.
            this.getAssociatedAntenna().getFrequencyHz(),
            distance,
        );
        this.setExpectedSignal(pr);
        let range = this.getExpectedRange();
        this.circle.setRadius(range);
        if (!no_rec) {
            this.associatedAntenna.updateExpectedSignal(true);
        }
    }
}
