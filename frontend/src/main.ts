import 'maplibre-gl/dist/maplibre-gl.css';
import './styles.css';
import {GeolocateControl, LngLat, Map as MapLibreMap, NavigationControl} from 'maplibre-gl';
import {DataProvider} from './DataProvider';
import {UIHelpers} from './uihelpers';
import {AntennaEnum} from './Antenna';
import {ChartController} from './controllers/ChartController';

const map = new MapLibreMap({
    container: 'map',
    center: [7.1731, 50.6596],
    zoom: 14,
    attributionControl: false,
    rollEnabled: true,
    style: {
        version: 8,
        sources: {
            'raster-tiles': {
                type: 'raster',
                tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],
                tileSize: 256,
                minzoom: 0,
                maxzoom: 19,
            },
        },
        layers: [
            {
                id: 'simple-tiles',
                type: 'raster',
                source: 'raster-tiles',
            },
        ],
    },
});

DataProvider.getSharedInstance().init(map);
UIHelpers.getInstance().init();
ChartController.getInstance().init();

const geolocate = new GeolocateControl({
    positionOptions: {
        enableHighAccuracy: true,
    },
    trackUserLocation: true,
    showAccuracyCircle: true,
    showUserLocation: true,
});
let navControl = new NavigationControl({
    showCompass: false,
    visualizePitch: false,
    showZoom: true,
});

map.addControl(navControl, 'top-right'); // Position in the bottom-left corner
map.addControl(geolocate, 'top-right'); // Position in the bottom-left corner

DataProvider.getSharedInstance().setMap(map);

// --- Context Menu for Antenna Position Selection ---

// Create context menu element
const contextMenu = document.createElement('div');
contextMenu.style.position = 'absolute';
contextMenu.style.display = 'none';
contextMenu.style.background = '#fff';
contextMenu.style.border = '1px solid #ccc';
contextMenu.style.borderRadius = '6px';
contextMenu.style.boxShadow = '0 2px 8px rgba(0,0,0,0.12)';
contextMenu.style.zIndex = '10000';
contextMenu.style.padding = '0';
contextMenu.innerHTML = `
    <button id="set-antenna-1" style="display:block;width:100%;padding:8px 18px;border:none;background:none;cursor:pointer;text-align:left;">Set Antenna 1 Position</button>
    <button id="set-antenna-2" style="display:block;width:100%;padding:8px 18px;border:none;background:none;cursor:pointer;text-align:left;">Set Antenna 2 Position</button>
`;
document.body.appendChild(contextMenu);

let contextMenuLngLat: LngLat | null = null;

// Show context menu on map right-click
map.on('contextmenu', (e) => {
    e.preventDefault();
    contextMenuLngLat = e.lngLat;
    contextMenu.style.left = `${e.originalEvent.clientX}px`;
    contextMenu.style.top = `${e.originalEvent.clientY}px`;
    contextMenu.style.display = 'block';
});

// Hide context menu on map click or drag
map.on('click', () => {
    contextMenu.style.display = 'none';
});
map.on('movestart', () => {
    contextMenu.style.display = 'none';
});
window.addEventListener('scroll', () => {
    contextMenu.style.display = 'none';
});
window.addEventListener('resize', () => {
    contextMenu.style.display = 'none';
});
document.addEventListener('click', (e) => {
    if (!contextMenu.contains(e.target as Node)) {
        contextMenu.style.display = 'none';
    }
});

// Handle menu button clicks
contextMenu.querySelector('#set-antenna-1')?.addEventListener('click', () => {
    if (contextMenuLngLat) {
        let current = DataProvider.getSharedInstance().getAntenna(AntennaEnum.ONE);
        current.setLatlng(contextMenuLngLat);
    }
    contextMenu.style.display = 'none';
});
contextMenu.querySelector('#set-antenna-2')?.addEventListener('click', () => {
    if (contextMenuLngLat) {
        let current = DataProvider.getSharedInstance().getAntenna(AntennaEnum.TWO);
        current.setLatlng(contextMenuLngLat);
    }
    contextMenu.style.display = 'none';
});

/**
 * Sets up event listeners for the antenna power, gain, frequency, and height input fields in the UI.
 * Updates the corresponding Antenna instance when a value changes.
 * @returns {void}
 */
function setupAntennaListeners() {
    const antenna1 = DataProvider.getSharedInstance().getAntenna(AntennaEnum.ONE);
    const antenna2 = DataProvider.getSharedInstance().getAntenna(AntennaEnum.TWO);

    if (!antenna1) {
        console.log('Antenna 1 not found');
    }
    if (!antenna2) {
        console.log('Antenna 2 not found');
    }

    // Left antenna (AntennaEnum.ONE)
    const txpwr1 = document.getElementById('device1-txpwr') as HTMLInputElement;
    const gain1 = document.getElementById('device1-gain') as HTMLInputElement;
    const freq1 = document.getElementById('device1-frequency') as HTMLInputElement;
    const height1 = document.getElementById('device1-height') as HTMLInputElement;
    // Right antenna (AntennaEnum.TWO)
    const txpwr2 = document.getElementById('device2-txpwr') as HTMLInputElement;
    const gain2 = document.getElementById('device2-gain') as HTMLInputElement;
    const freq2 = document.getElementById('device2-frequency') as HTMLInputElement;
    const height2 = document.getElementById('device2-height') as HTMLInputElement;

    if (txpwr1)
        txpwr1.addEventListener('change', () => {
            antenna1.setTransmittedPower(Number(txpwr1.value));
        });
    if (gain1)
        gain1.addEventListener('change', () => {
            antenna1.setAntennaGain(Number(gain1.value));
        });
    if (freq1)
        freq1.addEventListener('change', () => {
            antenna1.setFrequency(Number(freq1.value) * 1000);
        });
    if (height1)
        height1.addEventListener('change', () => {
            antenna1.setHeight(Number(height1.value));
        });
    if (txpwr2)
        txpwr2.addEventListener('change', () => {
            antenna2.setTransmittedPower(Number(txpwr2.value));
        });
    if (gain2)
        gain2.addEventListener('change', () => {
            antenna2.setAntennaGain(Number(gain2.value));
        });
    if (freq2)
        freq2.addEventListener('change', () => {
            antenna2.setFrequency(Number(freq2.value) * 1000);
        });
    if (height2)
        height2.addEventListener('change', () => {
            antenna2.setHeight(Number(height2.value));
        });
}

setupAntennaListeners();
