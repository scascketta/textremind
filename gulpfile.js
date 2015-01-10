var gulp = require('gulp');
var webserver = require('gulp-webserver');
var gutil = require('gulp-util');
var browserify = require('browserify');
var source = require('vinyl-source-stream');
var cssmin = require('gulp-cssmin');
var concat = require('gulp-concat');
var clean = require('gulp-clean');

gulp.task('clean', function() {
    return gulp.src('./dist/*')
        .pipe(clean());
});

gulp.task('browserify-app', function() {
    var bundler = browserify({
        debug: true,
        extensions: ['.js'],
        entries: ['./static/js/main.js'],
    })

    var bundle = function() {
        bundler
            .bundle()
            .on('error', function(e) {
                gutil.log(gutil.colors.red("something broke", e.toString()));
                this.emit('end');
            })
            .pipe(source('./static/js/bundle.js'))
            .pipe(gulp.dest('.'))
            .on('end', function() {
                gutil.log(gutil.colors.blue("browserify finished"));
            });
        return bundler
            .bundle()
            .on('error', function(e) {
                gutil.log(gutil.colors.red("something broke", e.toString()));
                this.emit('end');
            })
            .pipe(source('./dist/js/bundle.js'))
            .pipe(gulp.dest('.'))
            .on('end', function() {
                gutil.log(gutil.colors.blue("browserify finished"));
            });
    };

    return bundle();
});

gulp.task('build-css', function() {
    var src = [
        './static/css/normalize.css',
        './static/css/skeleton.css',
        './static/css/main.css',
    ];
    return gulp.src(src)
        .pipe(cssmin())
        .pipe(concat('bundle.css'))
        .pipe(gulp.dest('./dist/css'))
        .pipe(gulp.dest('./static/css'));
});

gulp.task('build-html', function() {
    var html = gulp.src('./static/index.html')
        .pipe(gulp.dest('./dist/'))
        .pipe(gulp.dest('./static'));
});

gulp.task('_serve', function() {
    return gulp.src('./dist/')
        .pipe(webserver({
            host: '0.0.0.0',
            port: 1234,
        }));
});

gulp.task('watch', function() {
    gulp.watch('./static/js/**', ['browserify-app']);
    gulp.watch('./static/css/**', ['build-css']);
    gulp.watch('./static/index.html', ['build-html']);
});

gulp.task('build', ['build-css', 'build-html', 'browserify-app']);
gulp.task('serve', ['build-css', 'build-html', 'browserify-app', '_serve', 'watch']);
