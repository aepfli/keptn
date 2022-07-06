import { ComponentFixture, TestBed } from '@angular/core/testing';
import { KtbStageDetailsComponent } from './ktb-stage-details.component';
import { HttpClientTestingModule } from '@angular/common/http/testing';
import { KtbStageDetailsModule } from './ktb-stage-details.module';
import { RouterTestingModule } from '@angular/router/testing';

describe('KtbStageDetailsComponent', () => {
  let component: KtbStageDetailsComponent;
  let fixture: ComponentFixture<KtbStageDetailsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [KtbStageDetailsModule, HttpClientTestingModule, RouterTestingModule],
    }).compileComponents();

    fixture = TestBed.createComponent(KtbStageDetailsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
