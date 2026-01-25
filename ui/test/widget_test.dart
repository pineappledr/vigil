import 'package:flutter_test/flutter_test.dart';
import 'package:ui/main.dart';

void main() {
  testWidgets('App smoke test', (WidgetTester tester) async {
    await tester.pumpWidget(const VigilApp());
    expect(find.text('Vigil'), findsOneWidget);
  });
}
