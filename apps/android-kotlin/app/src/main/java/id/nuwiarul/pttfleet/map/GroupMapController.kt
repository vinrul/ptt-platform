package id.nuwiarul.pttfleet.map

import android.os.Bundle
import id.nuwiarul.pttfleet.groups.GroupLocation
import org.maplibre.android.camera.CameraUpdateFactory
import org.maplibre.android.geometry.LatLng
import org.maplibre.android.geometry.LatLngBounds
import org.maplibre.android.maps.MapView
import org.maplibre.android.maps.MapLibreMap
import org.maplibre.android.maps.Style
import org.maplibre.android.style.layers.CircleLayer
import org.maplibre.android.style.layers.PropertyFactory.circleColor
import org.maplibre.android.style.layers.PropertyFactory.circleRadius
import org.maplibre.android.style.layers.PropertyFactory.circleStrokeColor
import org.maplibre.android.style.layers.PropertyFactory.circleStrokeWidth
import org.maplibre.android.style.sources.GeoJsonSource
import org.maplibre.android.style.layers.RasterLayer
import org.maplibre.android.style.sources.RasterSource
import org.maplibre.android.style.sources.TileSet
import org.maplibre.geojson.Feature
import org.maplibre.geojson.FeatureCollection
import org.maplibre.geojson.Point

class GroupMapController(
    private val mapView: MapView,
    private val onLocationSelected: (GroupLocation) -> Unit,
) {
    private var map: MapLibreMap? = null
    private val locations = mutableMapOf<String, GroupLocation>()

    fun onCreate(savedInstanceState: Bundle?) {
        mapView.onCreate(savedInstanceState)
        mapView.getMapAsync { readyMap ->
            map = readyMap
            readyMap.uiSettings.isLogoEnabled = false
            readyMap.uiSettings.isAttributionEnabled = false
            readyMap.setStyle(buildOsmStyle()) {
                readyMap.addOnMapClickListener { point ->
                    selectedLocationAt(point)?.let(onLocationSelected) != null
                }
                readyMap.addOnMapLongClickListener { point ->
                    selectedLocationAt(point)?.let(onLocationSelected) != null
                }
                readyMap.getStyle { style ->
                    updateSource(style)
                }
                renderAll(fitCamera = true)
            }
        }
    }

    fun replaceLocations(items: List<GroupLocation>) {
        locations.clear()
        items.forEach { locations[it.userId] = it }
        renderAll(fitCamera = true)
    }

    fun updateLocation(item: GroupLocation) {
        locations[item.userId] = item
        renderAll(fitCamera = false)
    }

    fun clear() {
        locations.clear()
        renderAll(fitCamera = false)
    }

    fun onStart() = mapView.onStart()

    fun onResume() = mapView.onResume()

    fun onPause() = mapView.onPause()

    fun onStop() = mapView.onStop()

    fun onLowMemory() = mapView.onLowMemory()

    fun onDestroy() = mapView.onDestroy()

    fun onSaveInstanceState(outState: Bundle) = mapView.onSaveInstanceState(outState)

    private fun renderAll(fitCamera: Boolean) {
        val activeMap = map ?: return
        activeMap.getStyle { style ->
            updateSource(style)
            if (fitCamera) fitCamera()
        }
    }

    private fun updateSource(style: Style) {
        val source = style.getSourceAs<GeoJsonSource>(USER_LOCATIONS_SOURCE_ID) ?: return
        source.setGeoJson(
            FeatureCollection.fromFeatures(
                locations.values.map(::locationFeature),
            ),
        )
    }

    private fun selectedLocationAt(point: LatLng): GroupLocation? {
        val activeMap = map ?: return null
        val screenPoint = activeMap.projection.toScreenLocation(point)
        val selectedUserId = activeMap.queryRenderedFeatures(
            screenPoint,
            USER_LOCATIONS_LAYER_ID,
        ).firstOrNull()?.getStringProperty(USER_ID_PROPERTY)
        return selectedUserId?.let(locations::get)
    }

    private fun locationFeature(item: GroupLocation): Feature =
        Feature.fromGeometry(Point.fromLngLat(item.lng, item.lat)).apply {
            addStringProperty(USER_ID_PROPERTY, item.userId)
            addStringProperty("username", item.username)
            addStringProperty("fullName", item.fullName)
            addStringProperty("role", item.role)
            addStringProperty("recordedAt", item.recordedAt)
        }

    private fun emptyLocations(): FeatureCollection =
        FeatureCollection.fromFeatures(emptyList<Feature>())

    private fun buildLocationsLayer(): CircleLayer {
        return CircleLayer(USER_LOCATIONS_LAYER_ID, USER_LOCATIONS_SOURCE_ID)
            .withProperties(
                circleRadius(8f),
                circleColor("#34D399"),
                circleStrokeColor("#06251A"),
                circleStrokeWidth(3f),
            )
    }

    private fun fitCamera() {
        val activeMap = map ?: return
        val points = locations.values.map { LatLng(it.lat, it.lng) }
        when (points.size) {
            0 -> Unit
            1 -> activeMap.animateCamera(CameraUpdateFactory.newLatLngZoom(points.first(), 14.0))
            else -> {
                val bounds = LatLngBounds.Builder()
                points.forEach(bounds::include)
                activeMap.animateCamera(
                    CameraUpdateFactory.newLatLngBounds(bounds.build(), 80),
                )
            }
        }
    }

    private fun buildOsmStyle(): Style.Builder {
        val tiles = TileSet(
            "2.1.0",
            "https://tile.openstreetmap.org/{z}/{x}/{y}.png",
        ).apply {
            minZoom = 0f
            maxZoom = 19f
        }
        return Style.Builder()
            .withSource(RasterSource(OSM_SOURCE_ID, tiles, 256))
            .withSource(GeoJsonSource(USER_LOCATIONS_SOURCE_ID, emptyLocations()))
            .withLayer(RasterLayer(OSM_LAYER_ID, OSM_SOURCE_ID))
            .withLayer(buildLocationsLayer())
    }

    private companion object {
        const val OSM_SOURCE_ID = "openstreetmap-source"
        const val OSM_LAYER_ID = "openstreetmap-layer"
        const val USER_LOCATIONS_SOURCE_ID = "user-locations-source"
        const val USER_LOCATIONS_LAYER_ID = "user-locations-layer"
        const val USER_ID_PROPERTY = "userId"
    }
}
