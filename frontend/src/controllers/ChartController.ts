/**
 * ChartController.ts - Chart rendering and update logic for terrain and antenna data
 * Contains: ChartController singleton for managing the Chart.js chart and updating datasets
 */

import Chart from 'chart.js/auto';
import {DataProvider} from '../DataProvider';
import {AntennaEnum} from '../Antenna';
import {UIHelpers} from '../uihelpers';

/**
 * Singleton controller for managing the terrain and antenna chart.
 */
export class ChartController {
    private static instance: ChartController | null = null;
    private chart: Chart;

    /**
     * Constructs the ChartController and initializes the Chart.js chart.
     */
    constructor() {
        this.chart = new Chart(document.getElementById('terrainchart') as HTMLCanvasElement, {
            data: {
                labels: [],
                datasets: [
                    {
                        type: 'line',
                        data: [],
                        order: 3,
                        pointStyle: false,
                    },
                    {
                        type: 'line',
                        data: [],
                        order: 2,
                        pointStyle: false,
                    },
                    {
                        type: 'scatter',
                        data: [],
                        order: 1,
                    },
                ],
            },
            options: {
                plugins: {
                    legend: {
                        display: false,
                    },
                },
                scales: {
                    x: {
                        type: 'linear',
                        position: 'bottom',
                        title: {
                            display: true,
                            text: 'Distance',
                        },
                    },
                    y: {},
                },
            },
        });
    }

    /**
     * Returns the singleton instance of ChartController.
     * @returns {ChartController} The singleton instance
     */
    static getInstance(): ChartController {
        if (!ChartController.instance) {
            ChartController.instance = new ChartController();
        }
        return ChartController.instance;
    }

    /**
     * Initializes the chart controller (currently does nothing).
     * @returns {void}
     */
    init(): void {}

    /**
     * Updates the chart with new data from the antennas.
     * @returns {Promise<void>} Resolves when the chart is updated
     */
    async update(): Promise<void> {
        let antenna_one = DataProvider.getSharedInstance().getAntenna(AntennaEnum.ONE);
        let antenna_two = DataProvider.getSharedInstance().getAntenna(AntennaEnum.TWO);

        if (!antenna_one || !antenna_two || !antenna_one.getLatlng() || !antenna_two.getLatlng()) {
            this.chart.data.datasets[0].data = [];
            this.chart.update();
            return;
        }

        let res = await fetch(
            `/api?latitude1=${antenna_one.getLatlng().lat}&longitude1=${antenna_one.getLatlng().lng}&latitude2=${antenna_two.getLatlng().lat}&longitude2=${antenna_two.getLatlng().lng}`,
        );
        let terrain = await res.json();
        terrain.sort((a, b) => a[0] - b[0]);
        let los: [number, number][] = [
            [0, terrain[0][1] + antenna_one.getHeight()],
            [terrain[terrain.length - 1][0], terrain[terrain.length - 1][1] + antenna_two.getHeight()],
        ];

        let collision_points: [number, number][] = UIHelpers.getCollisionPoints(terrain, (x) => {
            let start_y = terrain[0][1] + 4;
            let m =
                (terrain[terrain.length - 1][1] + antenna_two.getHeight() - (terrain[0][1] + antenna_one.getHeight())) /
                terrain[terrain.length - 1][0];
            return m * x + start_y;
        });
        this.updateChart(terrain, los, collision_points);
    }

    /**
     * Updates the chart data.
     * @param {[number, number][]} terrain - The terrain data points
     * @param {[number, number][]} los - The line-of-sight data points
     * @param {[number, number][]} [collision_points=[]] - The collision points data
     * @returns {void}
     */
    updateChart(terrain: [number, number][], los: [number, number][], collision_points: [number, number][] = []): void {
        this.chart.data.datasets[0].data = terrain;
        this.chart.data.datasets[1].data = los;
        this.chart.data.datasets[2].data = collision_points;

        this.chart.update();
    }
}
