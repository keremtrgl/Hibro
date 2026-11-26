import 'package:material_ui/material_ui.dart';

import '../api/models.dart';
import '../state/scope.dart';

/// Bottom sheet for sending a problem report about a stop and/or line.
/// Only the chosen kind, the stop/line ids, an optional plate and an
/// optional note are sent: no location, no device id.
Future<void> showReportSheet(BuildContext context, {Stop? stop, Line? line, String? lineId, String? plate}) {
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (_) => _ReportSheet(stop: stop, line: line, lineId: lineId ?? line?.id, plate: plate),
  );
}

class _ReportSheet extends StatefulWidget {
  const _ReportSheet({this.stop, this.line, this.lineId, this.plate});
  final Stop? stop;
  final Line? line;
  final String? lineId, plate;
  @override
  State<_ReportSheet> createState() => _ReportSheetState();
}

class _ReportSheetState extends State<_ReportSheet> {
  String? _kind;
  final _note = TextEditingController();
  bool _sending = false;

  List<String> get _kinds {
    final hasLine = widget.lineId != null;
    final hasStop = widget.stop != null;
    return [
      if (hasLine) ...['crowded', 'no_show', 'ghost', 'late', 'early', 'wrong_time'],
      if (hasStop) ...['wrong_stop', 'broken'] else if (hasLine) 'broken',
      'other',
    ];
  }

  @override
  void dispose() {
    _note.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final kind = _kind;
    if (kind == null) return;
    setState(() => _sending = true);
    final s = AppScope.read(context);
    final messenger = ScaffoldMessenger.of(context);
    try {
      await s.sendReport(kind: kind, stopId: widget.stop?.id, lineId: widget.lineId, plate: widget.plate, note: _note.text);
      if (!mounted) return;
      Navigator.pop(context);
      messenger.showSnackBar(const SnackBar(content: Text('Teşekkürler! Bildirimin veriyi düzeltmemize yardım edecek.')));
    } catch (e) {
      if (mounted) {
        setState(() => _sending = false);
        messenger.showSnackBar(SnackBar(content: Text(e.toString())));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context);
    final subject = [
      if (widget.line != null) '${widget.line!.short} ${widget.line!.name}',
      if (widget.stop != null) widget.stop!.name,
      if (widget.plate != null && widget.plate!.isNotEmpty) widget.plate!,
    ].join(' · ');
    return Padding(
      padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
      child: SafeArea(
        child: SingleChildScrollView(
          child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 0, 20, 4),
              child: Text('Sorun bildir', style: t.textTheme.titleLarge),
            ),
            if (subject.isNotEmpty)
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 0, 20, 8),
                child: Text(subject, style: t.textTheme.bodyMedium?.copyWith(color: t.colorScheme.onSurfaceVariant)),
              ),
            RadioGroup<String>(
              groupValue: _kind,
              onChanged: (v) => setState(() => _kind = v),
              child: Column(children: [
                for (final k in _kinds) RadioListTile<String>(value: k, title: Text(reportKinds[k]!), dense: true),
              ]),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 4, 20, 0),
              child: TextField(
                controller: _note,
                maxLength: 280,
                maxLines: 2,
                decoration: const InputDecoration(
                  labelText: 'Not (isteğe bağlı)',
                  hintText: 'Örn: Durak 50 m ileride, köşede',
                  border: OutlineInputBorder(),
                ),
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 0, 20, 4),
              child: Text(
                'Konumun ve kimliğin gönderilmez. Lütfen nota kişisel bilgi yazma.',
                style: t.textTheme.bodySmall?.copyWith(color: t.colorScheme.onSurfaceVariant),
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 8, 20, 16),
              child: SizedBox(
                width: double.infinity,
                child: FilledButton.icon(
                  onPressed: _kind == null || _sending ? null : _send,
                  icon: _sending
                      ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                      : const Icon(Icons.send),
                  label: const Text('Gönder'),
                ),
              ),
            ),
          ]),
        ),
      ),
    );
  }
}
