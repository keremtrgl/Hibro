import 'package:flutter/foundation.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';

/// System notifications, used for the "your stop is next" alarm so it is
/// noticed with the phone in a pocket or the screen locked.
class Notifier {
  static final _plugin = FlutterLocalNotificationsPlugin();
  static bool _ready = false;

  static const _alarmChannel = AndroidNotificationDetails(
    'stop_alarm',
    'İnilecek durak uyarısı',
    channelDescription: 'Yolculuk modunda ineceğin durağa yaklaşınca uyarır.',
    importance: Importance.max,
    priority: Priority.high,
    category: AndroidNotificationCategory.alarm,
    enableVibration: true,
    playSound: true,
    ticker: 'İnilecek durak yaklaşıyor',
  );

  static Future<void> init() async {
    if (kIsWeb || _ready) return;
    try {
      await _plugin.initialize(
        settings: const InitializationSettings(
          android: AndroidInitializationSettings('@mipmap/ic_launcher'),
          iOS: DarwinInitializationSettings(
            requestAlertPermission: false,
            requestSoundPermission: false,
            requestBadgePermission: false,
          ),
        ),
      );
      _ready = true;
    } catch (e) {
      debugPrint('notifications unavailable: $e');
    }
  }

  /// Asks for permission (Android 13+, iOS) when the user starts a ride,
  /// not at app start.
  static Future<void> requestPermission() async {
    if (!_ready) return;
    try {
      await _plugin.resolvePlatformSpecificImplementation<AndroidFlutterLocalNotificationsPlugin>()?.requestNotificationsPermission();
      await _plugin.resolvePlatformSpecificImplementation<IOSFlutterLocalNotificationsPlugin>()?.requestPermissions(alert: true, sound: true);
    } catch (_) {}
  }

  static Future<void> stopAlarm(String stopName, int meters) async {
    if (!_ready) return;
    try {
      await _plugin.show(
        id: 1001,
        title: 'İnilecek durak yaklaşıyor',
        body: '$stopName · yaklaşık $meters m kaldı',
        notificationDetails: const NotificationDetails(
          android: _alarmChannel,
          iOS: DarwinNotificationDetails(presentAlert: true, presentSound: true, interruptionLevel: InterruptionLevel.timeSensitive),
        ),
      );
    } catch (_) {}
  }
}
