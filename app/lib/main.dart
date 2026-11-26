import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/services.dart';
import 'package:maplibre_gl/maplibre_gl.dart' show setMaximumAmbientCacheSize;
import 'package:material_ui/material_ui.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'screens/alerts_screen.dart';
import 'screens/favorites_screen.dart';
import 'screens/home_screen.dart';
import 'screens/settings_screen.dart';
import 'state/app_state.dart';
import 'state/scope.dart';
import 'util/notify.dart';
import 'widgets/city_sheet.dart';
import 'widgets/consent_sheet.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  // Keep up to 150 MB of map tiles on the phone (default 50 MB) so the
  // user's own city is rarely downloaded twice. Not supported on web.
  if (!kIsWeb) {
    try {
      await setMaximumAmbientCacheSize(150 * 1024 * 1024);
    } catch (_) {}
  }
  await Notifier.init();
  final prefs = await SharedPreferences.getInstance();
  final state = AppState(prefs);
  runApp(AppScope(state: state, child: const GeliyoApp()));
  // Load the cached pack after the first frame so start-up stays instant.
  state.init();
}

class GeliyoApp extends StatelessWidget {
  const GeliyoApp({super.key});

  static const _seed = Color(0xFF00897B);

  ThemeData _theme(Brightness b) => ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: _seed, brightness: b),
        useMaterial3: true,
        listTileTheme: const ListTileThemeData(contentPadding: EdgeInsets.symmetric(horizontal: 16)),
      );

  @override
  Widget build(BuildContext context) {
    final s = AppScope.of(context);
    return MaterialApp(
      title: 'Geliyo',
      debugShowCheckedModeBanner: false,
      theme: _theme(Brightness.light),
      darkTheme: _theme(Brightness.dark),
      themeMode: switch (s.themeMode) {
        AppThemeMode.light => ThemeMode.light,
        AppThemeMode.dark => ThemeMode.dark,
        AppThemeMode.system => ThemeMode.system,
      },
      home: const _Shell(),
    );
  }
}

class _Shell extends StatefulWidget {
  const _Shell();
  @override
  State<_Shell> createState() => _ShellState();
}

class _ShellState extends State<_Shell> with WidgetsBindingObserver {
  int _tab = 0;
  static const _tabEvents = ['screen_map', 'screen_favorites', 'screen_alerts', 'screen_settings'];

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    WidgetsBinding.instance.addPostFrameCallback((_) => _firstRun());
  }

  /// First launch: privacy choices, then find the city from the location.
  /// No city data is downloaded until the city is known.
  Future<void> _firstRun() async {
    final s = AppScope.read(context);
    if (!s.consentAsked) await showConsentSheet(context);
    if (!mounted || s.hasCity) return;
    final r = await s.detectFromLocation();
    if (!mounted) return;
    final city = r.city;
    if (city != null) {
      await s.selectCity(city.id);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Konumuna göre ${city.name} seçildi.')));
      }
      return;
    }
    await showCitySheet(
      context,
      reason: r.located
          ? 'Bulunduğun şehir henüz desteklenmiyor. Başka bir şehre bakmak istersen seçebilirsin; hiçbir veri gereksiz yere indirilmez.'
          : 'Konum alınamadı. Şehrini elle seçebilirsin.',
    );
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  // Send pending anonymous counters when the app goes to the background.
  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.paused) AppScope.read(context).flushEvents();
  }

  @override
  Widget build(BuildContext context) {
    final dark = Theme.of(context).brightness == Brightness.dark;
    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: dark ? SystemUiOverlayStyle.light : SystemUiOverlayStyle.dark,
      child: Scaffold(
        body: IndexedStack(index: _tab, children: [
          HomeScreen(active: _tab == 0),
          const FavoritesScreen(),
          const AlertsScreen(),
          const SettingsScreen(),
        ]),
        bottomNavigationBar: NavigationBar(
          selectedIndex: _tab,
          onDestinationSelected: (i) {
            if (i != _tab) AppScope.read(context).track(_tabEvents[i]);
            setState(() => _tab = i);
          },
          labelBehavior: NavigationDestinationLabelBehavior.alwaysShow,
          destinations: const [
            NavigationDestination(icon: Icon(Icons.map_outlined), selectedIcon: Icon(Icons.map), label: 'Harita'),
            NavigationDestination(icon: Icon(Icons.star_border), selectedIcon: Icon(Icons.star), label: 'Favoriler'),
            NavigationDestination(icon: Icon(Icons.campaign_outlined), selectedIcon: Icon(Icons.campaign), label: 'Duyurular'),
            NavigationDestination(icon: Icon(Icons.settings_outlined), selectedIcon: Icon(Icons.settings), label: 'Ayarlar'),
          ],
        ),
      ),
    );
  }
}
