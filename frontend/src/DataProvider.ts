/**
 * @fileoverview DataProvider - Central data management for antennas, map, and event system.
 * Contains: DataProvider singleton, event system, antenna and marker management.
 */

import {Map as MapLibreMap} from 'maplibre-gl';
import {Antenna, AntennaEnum} from './Antenna';

/**
 * Type for event listeners in DataProvider.
 * @param event - The event type from DataProviderEventEnum
 * @param data - Optional event data
 */
export type DataProviderEventListener = (event: DataProviderEventEnum, data?: any) => void;

/**
 * Enum for supported DataProvider events.
 * @readonly
 * @enum {string}
 */
export enum DataProviderEventEnum {
    ANTENNA_POSITION_CHANGED = 'antennaPositionChanged',
}

export type DataProviderEventData = {
    antenna: Antenna;
    antenna_id: AntennaEnum;
};

/**
 * Singleton class for managing antennas, map, and event listeners.
 */
export class DataProvider {
    /**
     * The singleton instance of DataProvider.
     * @type {DataProvider | null}
     */
    static instance: DataProvider | null = null;
    /**
     * Map of antennas by AntennaEnum.
     * @type {Map<AntennaEnum, Antenna>}
     */
    private antennas: Map<AntennaEnum, Antenna> = new Map<AntennaEnum, Antenna>();

    /**
     * The map instance.
     * @type {MapLibreMap | null}
     */
    private map: MapLibreMap | null = null;
    /**
     * Event listeners by event type.
     * @type {Map<DataProviderEventEnum, DataProviderEventListener[]>}
     */
    private listeners: Map<DataProviderEventEnum, DataProviderEventListener[]> = new Map<
        DataProviderEventEnum,
        DataProviderEventListener[]
    >();

    /**
     * Constructor for DataProvider (private for singleton).
     */
    constructor() {}

    /**
     * Returns the singleton instance of DataProvider.
     * @returns {DataProvider} The shared DataProvider instance
     */
    public static getSharedInstance(): DataProvider {
        if (!DataProvider.instance) {
            DataProvider.instance = new DataProvider();
        }
        return DataProvider.instance;
    }

    /**
     * Initializes DataProvider (placeholder).
     * @returns {void}
     */
    public init(map: MapLibreMap): void {
        const antenna_one = new Antenna(AntennaEnum.ONE);
        const antenna_two = new Antenna(AntennaEnum.TWO);
        this.setAntenna(AntennaEnum.ONE, antenna_one);
        this.setAntenna(AntennaEnum.TWO, antenna_two);

        antenna_one.addToMap(map);
        antenna_two.addToMap(map);
        antenna_one.setAssociatedAntenna(antenna_two);
        antenna_two.setAssociatedAntenna(antenna_one);
        antenna_one.setDeviceName('Antenna One (Blue)');
        antenna_two.setDeviceName('Antenna Two (Pink)');
        antenna_one.setHeight(5);
        antenna_two.setHeight(6);
        antenna_one.setTransmittedPower(20);
        antenna_two.setTransmittedPower(21);
        antenna_one.setAntennaGain(10);
        antenna_two.setAntennaGain(11);
        antenna_one.setFrequency(240000);
        antenna_two.setFrequency(240000);
        antenna_one.setSensitivity(-100);
        antenna_two.setSensitivity(-100);
    }

    /**
     * Returns the Antenna instance for the given enum.
     * @param {AntennaEnum} antenna - The antenna identifier
     * @returns {Antenna | undefined} The Antenna instance or undefined
     */
    public getAntenna(antenna: AntennaEnum): Antenna {
        return this.antennas.get(antenna);
    }

    /**
     * Sets the Antenna instance for the given enum and emits an event.
     * @param {AntennaEnum} id - The antenna identifier
     * @param {Antenna} antenna - The Antenna instance
     * @returns {void}
     */
    public setAntenna(id: AntennaEnum, antenna: Antenna): void {
        this.antennas.set(id, antenna);
        this.emitEvent(DataProviderEventEnum.ANTENNA_POSITION_CHANGED, {
            antenna: antenna,
            antenna_id: id,
        });
    }

    /**
     * Adds an event listener for a specific event type.
     * @param {DataProviderEventEnum} event - The event type
     * @param {DataProviderEventListener} listener - The event listener function
     * @returns {void}
     */
    public addEventListener(event: DataProviderEventEnum, listener: DataProviderEventListener): void {
        if (!this.listeners.has(event)) {
            this.listeners.set(event, []);
        }
        this.listeners.get(event).push(listener);
    }

    /**
     * Emits an event to all registered listeners for the event type.
     * @param {DataProviderEventEnum} event - The event type
     * @param {any} [data] - Optional event data
     * @returns {void}
     */
    public emitEvent(event: DataProviderEventEnum, data?: any): void {
        if (!this.listeners.has(event)) return;
        for (const listener of this.listeners.get(event)) {
            listener(event, data);
        }
    }

    /**
     * Sets the map instance for the DataProvider.
     * @param {MapLibreMap} map - The map instance
     * @returns {void}
     */
    setMap(map: MapLibreMap): void {
        this.map = map;
    }

    /**
     * Gets the current map instance.
     * @returns {MapLibreMap | null} The map instance or null
     */
    getMap(): MapLibreMap | null {
        return this.map;
    }
}
