import 'package:get_it/get_it.dart';

import 'auth_cubit.dart';
import 'auth_service.dart';

final getIt = GetIt.instance;

void setup() {
  getIt.registerSingleton<AuthService>(AuthService());
  getIt.registerFactory(() => AuthCubit(getIt<AuthService>()));
}