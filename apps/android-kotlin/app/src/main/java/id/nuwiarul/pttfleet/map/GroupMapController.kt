package id.nuwiarul.pttfleet.map

import android.os.Bundle
import id.nuwiarul.pttfleet.groups.GroupLocation
import org.maplibre.android.annotations.Marker
import org.maplibre.android.annotations.MarkerOptions
import org.maplibre.android.camera.CameraUpdateFactory
import org.maplibre.android.geometry.LatLng
import org.maplibre.android.geometry.LatLngBounds
import org.maplibre.android.maps.MapView
import org.maplibre.android.maps.MapLibreMap
import org.maplibre.android.maps.Style
import org.maplibre.android.style.layers.RasterLayer
import org.maplibre.android.style.sources.RasterSource
import org.maplibre.android.style.sources.TileSet

class GroupMapController(
    private val mapView: MapView,
    private val onLocationSelected: (GroupLocation) -> Unit,
) {
    private var map: MapLibreMap? = null
    private val locations = mutableMapOf<String, GroupLocation>()
    private val markersByUserId = mutableMapOf<String, Marker>()
    private val userIdsByMarkerId = mutableMapOf<Long, String>()

    fun onCreate(savedInstanceState: Bundle?) {
        mapView.onCreate(savedInstanceState)
        mapView.getMapAsync { readyMap ->
            map = readyMap
            readyMap.uiSettings.isLogoEnabled = false
            readyMap.uiSettings.isAttributionEnabled = false
            readyMap.setStyle(buildOsmStyle()) {
                readyMap.setOnMarkerClickListener { marker ->
                    userIdsByMarkerId[marker.id]
                        ?.let(locations::get)
                        ?.let(onLocationSelected)
                    true
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
        val marker = markersByUserId[item.userId]
        if (marker == null) {
            addMarker(item)
        } else {
            marker.position = LatLng(item.lat, item.lng)
            marker.title = item.fullName
            marker.snippet = "@${item.username}"
        }
    }

    fun clear() {
        locations.clear()
        val activeMap = map ?: return
        markersByUserId.values.forEach(activeMap::removeMarker)
        markersByUserId.clear()
        userIdsByMarkerId.clear()
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
        val removedUserIds = markersByUserId.keys - locations.keys
        removedUserIds.forEach { userId ->
            markersByUserId.remove(userId)?.let { marker ->
                userIdsByMarkerId.remove(marker.id)
                activeMap.removeMarker(marker)
            }
        }
        locations.values.forEach { item ->
            val marker = markersByUserId[item.userId]
            if (marker == null) {
                addMarker(item)
            } else {
                marker.position = LatLng(item.lat, item.lng)
                marker.title = item.fullName
                marker.snippet = "@${item.username}"
            }
        }
        if (fitCamera) fitCamera()
    }

    private fun addMarker(item: GroupLocation) {
        val activeMap = map ?: return
        val marker = activeMap.addMarker(
            MarkerOptions()
                .position(LatLng(item.lat, item.lng))
                .title(item.fullName)
                .snippet("@${item.username}"),
        )
        markersByUserId[item.userId] = marker
        userIdsByMarkerId[marker.id] = item.userId
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
            .withLayer(RasterLayer(OSM_LAYER_ID, OSM_SOURCE_ID))
    }

    private companion object {
        const val OSM_SOURCE_ID = "openstreetmap-source"
        const val OSM_LAYER_ID = "openstreetmap-layer"
    }
}
