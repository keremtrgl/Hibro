import 'package:material_ui/material_ui.dart';

import '../api/models.dart';
import '../util/style.dart';

/// Coloured pill with the line number, e.g. [ 5 ] or [ M1 ].
class LineBadge extends StatelessWidget {
  const LineBadge({super.key, required this.text, required this.mode, this.color = '', this.textColor = '', this.dense = false});
  final String text, color, textColor;
  final Mode mode;
  final bool dense;

  @override
  Widget build(BuildContext context) {
    final bg = hexColor(color, modeColor(mode));
    final fg = textColor.isNotEmpty
        ? hexColor(textColor, Colors.white)
        : (bg.computeLuminance() > 0.6 ? Colors.black : Colors.white);
    return Container(
      constraints: BoxConstraints(minWidth: dense ? 34 : 44),
      padding: EdgeInsets.symmetric(horizontal: dense ? 6 : 8, vertical: dense ? 2 : 4),
      decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(6)),
      child: Text(
        text,
        textAlign: TextAlign.center,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(color: fg, fontWeight: FontWeight.w700, fontSize: dense ? 12 : 14),
      ),
    );
  }
}

class ModeAvatar extends StatelessWidget {
  const ModeAvatar({super.key, required this.mode, this.size = 36});
  final Mode mode;
  final double size;
  @override
  Widget build(BuildContext context) => Container(
        width: size,
        height: size,
        decoration: BoxDecoration(color: modeColor(mode).withValues(alpha: 0.14), shape: BoxShape.circle),
        child: Icon(modeIcon(mode), color: modeColor(mode), size: size * 0.56),
      );
}

/// Small pulsing dot that marks live (GPS based) information.
class LiveDot extends StatefulWidget {
  const LiveDot({super.key});
  @override
  State<LiveDot> createState() => _LiveDotState();
}

class _LiveDotState extends State<LiveDot> with SingleTickerProviderStateMixin {
  late final AnimationController _c = AnimationController(vsync: this, duration: const Duration(milliseconds: 1200))
    ..repeat(reverse: true);
  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => FadeTransition(
        opacity: Tween(begin: 0.35, end: 1.0).animate(_c),
        child: Container(width: 8, height: 8, decoration: const BoxDecoration(color: Color(0xFF2E7D32), shape: BoxShape.circle)),
      );
}

class EmptyState extends StatelessWidget {
  const EmptyState({super.key, required this.icon, required this.title, this.body, this.action});
  final IconData icon;
  final String title;
  final String? body;
  final Widget? action;
  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          Icon(icon, size: 48, color: t.colorScheme.outline),
          const SizedBox(height: 12),
          Text(title, style: t.textTheme.titleMedium, textAlign: TextAlign.center),
          if (body != null) ...[
            const SizedBox(height: 6),
            Text(body!, style: t.textTheme.bodyMedium?.copyWith(color: t.colorScheme.onSurfaceVariant), textAlign: TextAlign.center),
          ],
          if (action != null) ...[const SizedBox(height: 16), action!],
        ]),
      ),
    );
  }
}
