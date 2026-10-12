//go:build linux

package menubar

/*
#cgo linux pkg-config: gtk+-3.0
#include <gtk/gtk.h>

static gboolean on_context_menu_cb(GtkWidget *widget, gpointer context_menu, gpointer event, gpointer hit_test_result, gpointer user_data) {
    return TRUE; // Returning TRUE cancels the context menu completely
}

static void find_and_disable_context_menu(GtkWidget *widget, gpointer data) {
    if (!widget) return;
    GType type = G_OBJECT_TYPE(widget);
    const gchar *type_name = g_type_name(type);
    if (type_name && g_str_has_prefix(type_name, "WebKitWebView")) {
        g_signal_handlers_disconnect_by_func(widget, G_CALLBACK(on_context_menu_cb), NULL);
        g_signal_connect(widget, "context-menu", G_CALLBACK(on_context_menu_cb), NULL);
        return;
    }
    if (GTK_IS_CONTAINER(widget)) {
        gtk_container_forall(GTK_CONTAINER(widget), find_and_disable_context_menu, data);
    }
}

static gboolean disable_context_menus_idle(gpointer data) {
    GList *toplevels = gtk_window_list_toplevels();
    for (GList *l = toplevels; l != NULL; l = l->next) {
        if (GTK_IS_WINDOW(l->data)) {
            find_and_disable_context_menu(GTK_WIDGET(l->data), data);
        }
    }
    g_list_free(toplevels);
    return G_SOURCE_REMOVE;
}

static void disable_context_menu() {
    g_idle_add(disable_context_menus_idle, NULL);
}

static void find_and_set_menubar_visibility(GtkWidget *widget, gpointer data) {
    if (!widget) return;
    gboolean visible = GPOINTER_TO_INT(data);
    if (GTK_IS_MENU_BAR(widget)) {
        if (visible) {
            gtk_widget_set_no_show_all(widget, FALSE);
            gtk_widget_show(widget);
        } else {
            gtk_widget_set_no_show_all(widget, TRUE);
            gtk_widget_hide(widget);
        }
        return;
    }
    if (GTK_IS_CONTAINER(widget)) {
        gtk_container_forall(GTK_CONTAINER(widget), find_and_set_menubar_visibility, data);
    }
}

static gboolean set_all_menubars_visible_cb(gpointer data) {
    GList *toplevels = gtk_window_list_toplevels();
    for (GList *l = toplevels; l != NULL; l = l->next) {
        if (GTK_IS_WINDOW(l->data)) {
            find_and_set_menubar_visibility(GTK_WIDGET(l->data), data);
        }
    }
    g_list_free(toplevels);
    return G_SOURCE_REMOVE;
}

static void set_menubars_visible(int visible) {
    g_idle_add(set_all_menubars_visible_cb, GINT_TO_POINTER(visible));
    // Re-verify shortly to overcome window manager fullscreen re-map race conditions
    g_timeout_add(100, set_all_menubars_visible_cb, GINT_TO_POINTER(visible));
    g_timeout_add(300, set_all_menubars_visible_cb, GINT_TO_POINTER(visible));
}
*/
import "C"

// DisableContextMenu disables native WebKitGTK right-click context menus.
func DisableContextMenu() {
	C.disable_context_menu()
}

// SetNativeMenubarVisible toggles the visibility of the native GTK menubar on Linux.
func SetNativeMenubarVisible(visible bool) {
	if visible {
		C.set_menubars_visible(1)
	} else {
		C.set_menubars_visible(0)
	}
}
