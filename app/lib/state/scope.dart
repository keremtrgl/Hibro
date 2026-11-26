import 'package:material_ui/material_ui.dart';

import 'app_state.dart';

/// Makes [AppState] available to the widget tree and rebuilds dependents
/// when it notifies. No third-party state library needed.
class AppScope extends InheritedNotifier<AppState> {
  const AppScope({super.key, required AppState state, required super.child}) : super(notifier: state);

  static AppState of(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<AppScope>()!.notifier!;

  static AppState read(BuildContext context) =>
      context.getInheritedWidgetOfExactType<AppScope>()!.notifier!;
}
